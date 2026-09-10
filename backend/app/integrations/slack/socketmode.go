package slack

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/slack-go/slack/socketmode"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const (
	maxReconnectDelay = time.Minute
	socketEventBudget = 30 * time.Second
)

// Manager runs one Socket Mode connection per enabled integration that has
// an app-level token. It reconciles against the integrations table on an
// interval and whenever the agent package reports a change, so enabling,
// disabling, re-keying or deleting an integration starts or stops its loop.
type Manager struct {
	app      *App
	interval time.Duration
	kick     chan struct{}

	mu    sync.Mutex
	loops map[int]*loop
}

type loop struct {
	fingerprint string
	cancel      context.CancelFunc
	done        chan struct{}
}

// StartSocketMode starts the manager and returns it; it stops with ctx.
func (a *App) StartSocketMode(ctx context.Context, interval time.Duration) *Manager {
	m := &Manager{app: a, interval: interval, kick: make(chan struct{}, 1), loops: map[int]*loop{}}
	agent.OnIntegrationsChanged(m.Kick)
	go m.run(ctx)
	return m
}

// Kick asks for a reconcile before the next tick.
func (m *Manager) Kick() {
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

// Running lists the integration ids with a live loop.
func (m *Manager) Running() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int, 0, len(m.loops))
	for id := range m.loops {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) run(ctx context.Context) {
	defer traceway.Recover()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	m.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			return
		case <-ticker.C:
			m.reconcile(ctx)
		case <-m.kick:
			m.reconcile(ctx)
		}
	}
}

// Reconcile starts loops for integrations that should have one and stops
// the rest. Exported for tests; the manager calls it on its own schedule.
func (m *Manager) Reconcile(ctx context.Context) {
	m.reconcile(ctx)
}

func (m *Manager) reconcile(ctx context.Context) {
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Integration, error) {
		return transactional.IntegrationRepository.FindAllByProvider(tx, Provider)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("slack socket mode: list integrations: %w", err))
		return
	}
	desired := map[int]struct {
		in *models.Integration
		s  settings
	}{}
	for _, in := range rows {
		if !in.Enabled {
			continue
		}
		s, err := m.app.settings(in)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("slack socket mode: integration %d config: %w", in.Id, err))
			continue
		}
		if s.AppToken == "" {
			continue
		}
		desired[in.Id] = struct {
			in *models.Integration
			s  settings
		}{in, s}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for id, running := range m.loops {
		want, ok := desired[id]
		if ok && running.fingerprint == fingerprint(want.s) {
			continue
		}
		running.cancel()
		<-running.done
		delete(m.loops, id)
	}
	for id, want := range desired {
		if _, ok := m.loops[id]; ok {
			continue
		}
		loopCtx, cancel := context.WithCancel(ctx)
		l := &loop{fingerprint: fingerprint(want.s), cancel: cancel, done: make(chan struct{})}
		m.loops[id] = l
		go m.serve(loopCtx, want.in, want.s, l.done)
	}
}

func (m *Manager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, running := range m.loops {
		running.cancel()
		<-running.done
		delete(m.loops, id)
	}
}

func fingerprint(s settings) string {
	sum := sha256.Sum256([]byte(s.AppToken + "\x00" + s.BotToken))
	return hex.EncodeToString(sum[:8])
}

// serve keeps one integration connected until its context ends, backing
// off when the connection cannot be established or drops for good.
func (m *Manager) serve(ctx context.Context, in *models.Integration, s settings, done chan struct{}) {
	defer close(done)
	defer traceway.Recover()
	delay := time.Second
	for ctx.Err() == nil {
		err := m.connect(ctx, in, s)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("slack socket mode: integration %d disconnected: %v (retrying in %s)", in.Id, err, delay)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxReconnectDelay)
	}
}

func (m *Manager) connect(ctx context.Context, in *models.Integration, s settings) error {
	client := socketmode.New(m.app.api(s))
	runErr := make(chan error, 1)
	go func() {
		defer traceway.Recover()
		runErr <- client.RunContext(ctx)
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-runErr:
			return err
		case event := <-client.Events:
			m.handle(ctx, client, in, event)
		}
	}
}

func (m *Manager) handle(ctx context.Context, client *socketmode.Client, in *models.Integration, event socketmode.Event) {
	if event.Type == socketmode.EventTypeConnectionError {
		log.Printf("slack socket mode: integration %d connection error: %v", in.Id, event.Data)
		return
	}
	env, ok := fromSocket(event)
	if !ok {
		return
	}
	if event.Request != nil {
		if err := client.Ack(*event.Request); err != nil {
			traceway.CaptureException(fmt.Errorf("slack socket mode: ack: %w", err))
		}
	}
	eventCtx, cancel := context.WithTimeout(ctx, socketEventBudget)
	defer cancel()
	if _, err := m.app.dispatch(eventCtx, in, env); err != nil {
		traceway.CaptureException(fmt.Errorf("slack socket mode: integration %d event %s: %w", in.Id, env.kind, err))
	}
}
