package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"golang.org/x/sync/singleflight"
)

const (
	// sdkConfigTTL bounds how stale a replica can be if a change notification is lost.
	sdkConfigTTL = 2 * time.Minute
)

// SdkConfigEntry is a project's compiled rules and captures. Version changes on every load, so callers can memoize
// rendered documents per entry.
type SdkConfigEntry struct {
	Compiled *sdkconfig.Compiled
	Version  uint64
	loadedAt time.Time
}

type sdkConfigCache struct {
	mu         sync.RWMutex
	entries    map[uuid.UUID]*SdkConfigEntry
	group      singleflight.Group
	version    uint64
	generation uint64
	load       func(projectId uuid.UUID) (*sdkconfig.Compiled, error)
	now        func() time.Time
}

// SdkConfigCache serves every OpAMP poll, so it never touches the database on the hot path. A write invalidates
// the project here and, on Postgres, on every other replica through NOTIFY.
var SdkConfigCache = &sdkConfigCache{
	entries: map[uuid.UUID]*SdkConfigEntry{},
	load:    loadSdkConfig,
	now:     time.Now,
}

func (c *sdkConfigCache) Get(projectId uuid.UUID) (*SdkConfigEntry, error) {
	c.mu.RLock()
	entry, ok := c.entries[projectId]
	generation := c.generation
	c.mu.RUnlock()
	if ok && c.now().Sub(entry.loadedAt) < sdkConfigTTL {
		return entry, nil
	}
	loaded, err, _ := c.group.Do(fmt.Sprintf("%s:%d", projectId, generation), func() (any, error) {
		compiled, err := c.load(projectId)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.version++
		entry := &SdkConfigEntry{Compiled: compiled, Version: c.version, loadedAt: c.now()}
		if c.generation == generation {
			c.entries[projectId] = entry
		}
		return entry, nil
	})
	if err != nil {
		return nil, err
	}
	return loaded.(*SdkConfigEntry), nil
}

func (c *sdkConfigCache) Invalidate(projectId uuid.UUID) {
	c.mu.Lock()
	delete(c.entries, projectId)
	c.generation++
	c.mu.Unlock()
}

func (c *sdkConfigCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = map[uuid.UUID]*SdkConfigEntry{}
	c.generation++
	c.mu.Unlock()
}

// Init starts listening for changes made on other replicas.
func (c *sdkConfigCache) Init(ctx context.Context) error {
	return c.startListener(ctx)
}

func loadSdkConfig(projectId uuid.UUID) (*sdkconfig.Compiled, error) {
	type rows struct {
		rules    []*models.SdkConfigRule
		captures []*models.SdkCapture
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (rows, error) {
		rules, err := transactional.SdkConfigRepository.ListRules(tx, projectId)
		if err != nil {
			return rows{}, err
		}
		captures, err := transactional.SdkConfigRepository.ListActiveCaptures(tx, projectId, time.Now().UTC())
		return rows{rules: rules, captures: captures}, err
	})
	if err != nil {
		return nil, fmt.Errorf("loading SDK config for project %s: %w", projectId, err)
	}
	return CompileSdkConfig(loaded.rules, loaded.captures, time.Now()), nil
}

// CompileSdkConfig turns stored rules and captures into their evaluable form. Rows were validated on write; one that
// no longer parses (a schema change) is skipped and reported rather than failing every poll of the project.
func CompileSdkConfig(rules []*models.SdkConfigRule, captures []*models.SdkCapture, now time.Time) *sdkconfig.Compiled {
	compiled := &sdkconfig.Compiled{}
	for _, row := range rules {
		if !row.Enabled {
			continue
		}
		conditions, err := sdkconfig.ParseConditions(row.Conditions)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("skipping SDK config rule %d: conditions: %w", row.Id, err))
			continue
		}
		settings, err := sdkconfig.ParseSettings(row.Settings)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("skipping SDK config rule %d: settings: %w", row.Id, err))
			continue
		}
		compiled.Rules = append(compiled.Rules, sdkconfig.Rule{
			Id: "rule:" + strconv.Itoa(row.Id), Name: row.Name, Conditions: conditions, Settings: settings,
		})
	}
	for _, row := range captures {
		if row.CancelledAt != nil || !row.ExpiresAt.After(now) {
			continue
		}
		conditions, err := sdkconfig.ParseConditions(row.Conditions)
		if err != nil {
			traceway.CaptureException(fmt.Errorf("skipping SDK capture %s: conditions: %w", row.Id, err))
			continue
		}
		var actions sdkconfig.CaptureActions
		if len(row.Actions) > 0 {
			if err := json.Unmarshal(row.Actions, &actions); err != nil {
				traceway.CaptureException(fmt.Errorf("skipping SDK capture %s: actions: %w", row.Id, err))
				continue
			}
		}
		compiled.Captures = append(compiled.Captures, sdkconfig.ActiveCapture{
			Id: row.Id.String(), Conditions: conditions, ExpiresAt: row.ExpiresAt,
			LookbackSeconds: actions.LookbackSeconds, Signals: actions.Signals, Replay: actions.Replay,
		})
	}
	return compiled
}
