package recordings

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/storage"
	traceway "go.tracewayapp.com"
)

const (
	defaultWorkers      = 32
	defaultQueueSize    = 2048
	metricsTickInterval = 10 * time.Second
	dropReportInterval  = time.Minute
	insertBatchSize     = 1000
	insertFlushInterval = 2 * time.Second
)

type Job struct {
	Id           uuid.UUID
	ProjectId    uuid.UUID
	ExceptionId  uuid.UUID
	SessionId    *uuid.UUID
	SegmentIndex int32
	Key          string
	Body         []byte
	RecordedAt   time.Time
}

type storer interface {
	Write(ctx context.Context, key string, data []byte) error
}

type pool struct {
	workers int
	jobs    chan Job
	inserts chan models.SessionRecording

	storage storer

	inFlight atomic.Int64
	uploaded atomic.Uint64
	dropped  atomic.Uint64
	failed   atomic.Uint64

	dropMu             sync.Mutex
	lastDropReportAt   time.Time
	droppedSinceReport uint64
}

var singleton *pool

func Start(ctx context.Context) {
	if singleton != nil {
		return
	}

	workers := defaultWorkers
	workersStr := strings.TrimSpace(config.Config.SessionRecordingUploadWorkers)
	if workersStr != "" {
		if v, err := strconv.Atoi(workersStr); err == nil && v >= 0 {
			workers = v
		}
	}

	queueSize := defaultQueueSize
	queueStr := strings.TrimSpace(config.Config.SessionRecordingUploadQueueSize)
	if queueStr != "" {
		if v, err := strconv.Atoi(queueStr); err == nil && v >= 1 {
			queueSize = v
		}
	}

	singleton = &pool{
		workers: workers,
		jobs:    make(chan Job, queueSize),
		inserts: make(chan models.SessionRecording, insertBatchSize),
		storage: storage.Store,
	}
	singleton.start(ctx)

	if workers == 0 {
		config.Logln("Session recording uploader disabled (SESSION_RECORDING_UPLOAD_WORKERS=0); segments will be dropped")
	} else {
		config.Logf("Started session recording uploader (workers=%d, queue=%d)", workers, queueSize)
	}
}

func Enqueue(j Job) {
	if singleton == nil {
		return
	}
	singleton.enqueue(j)
}

func (p *pool) start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		go p.worker(ctx)
	}
	go p.metricsLoop(ctx)
	go p.batcher(ctx)
}

func (p *pool) enqueue(j Job) {
	select {
	case p.jobs <- j:
		return
	default:
	}

	p.dropped.Add(1)

	var report uint64
	p.dropMu.Lock()
	p.droppedSinceReport++
	if time.Since(p.lastDropReportAt) >= dropReportInterval {
		report = p.droppedSinceReport
		p.droppedSinceReport = 0
		p.lastDropReportAt = time.Now()
	}
	p.dropMu.Unlock()

	if report > 0 {
		traceway.CaptureException(traceway.NewStackTraceErrorf(
			"session recording uploader dropped %d segments due to full queue (cap=%d)", report, cap(p.jobs)))
	}
}

func (p *pool) worker(ctx context.Context) {
	defer traceway.Recover()

	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-p.jobs:
			if !ok {
				return
			}
			p.handle(ctx, j)
		}
	}
}

func (p *pool) handle(ctx context.Context, j Job) {
	p.inFlight.Add(1)
	defer p.inFlight.Add(-1)

	if err := p.storage.Write(ctx, j.Key, j.Body); err != nil {
		p.failed.Add(1)
		traceway.CaptureException(traceway.NewStackTraceErrorf("failed to write session recording (key=%s): %w", j.Key, err))
		return
	}

	p.uploaded.Add(1)

	rec := models.SessionRecording{
		Id:           j.Id,
		ProjectId:    j.ProjectId,
		ExceptionId:  j.ExceptionId,
		SessionId:    j.SessionId,
		SegmentIndex: j.SegmentIndex,
		FilePath:     j.Key,
		RecordedAt:   j.RecordedAt,
	}
	select {
	case p.inserts <- rec:
	case <-ctx.Done():
	}
}

func (p *pool) batcher(ctx context.Context) {
	defer traceway.Recover()

	batch := make([]models.SessionRecording, 0, insertBatchSize)
	timer := time.NewTimer(insertFlushInterval)
	if !timer.Stop() {
		<-timer.C
	}
	timerActive := false

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := telemetry.SessionRecordingRepository.InsertAsync(ctx, batch); err != nil {
			p.failed.Add(uint64(len(batch)))
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to insert batch of %d session recording rows: %w", len(batch), err))
		}
		batch = batch[:0]
		if timerActive {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timerActive = false
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case rec, ok := <-p.inserts:
			if !ok {
				flush()
				return
			}
			batch = append(batch, rec)
			if !timerActive {
				timer.Reset(insertFlushInterval)
				timerActive = true
			}
			if len(batch) >= insertBatchSize {
				flush()
			}
		case <-timer.C:
			timerActive = false
			flush()
		}
	}
}

func (p *pool) metricsLoop(ctx context.Context) {
	defer traceway.Recover()

	var prevUploaded, prevDropped, prevFailed uint64

	ticker := time.NewTicker(metricsTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			curUploaded := p.uploaded.Load()
			curDropped := p.dropped.Load()
			curFailed := p.failed.Load()
			monitoring.RecordRecordingUploader(
				len(p.jobs),
				int(p.inFlight.Load()),
				curUploaded-prevUploaded,
				curDropped-prevDropped,
				curFailed-prevFailed,
			)
			prevUploaded = curUploaded
			prevDropped = curDropped
			prevFailed = curFailed
		}
	}
}
