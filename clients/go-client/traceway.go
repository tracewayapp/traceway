package traceway

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"traceway/metrics/cpu"
	"traceway/metrics/mem"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Context key for scope
type tracewayContextKey string

const CtxScopeKey tracewayContextKey = "TRACEWAY_SCOPE"
const CtxTransactionKey tracewayContextKey = "TRACEWAY_TRANSACTION"

// Scope holds contextual data for exceptions and transactions
type Scope struct {
	tags map[string]string
	mu   sync.RWMutex
}

// NewScope creates a new empty scope
func NewScope() *Scope {
	return &Scope{tags: make(map[string]string)}
}

// SetTag sets a tag on the scope
func (s *Scope) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[key] = value
}

// GetTag gets a tag from the scope
func (s *Scope) GetTag(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.tags[key]
	return val, ok
}

// GetTags returns a copy of all tags
func (s *Scope) GetTags() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.tags))
	for k, v := range s.tags {
		result[k] = v
	}
	return result
}

// Clone creates a deep copy of the scope
func (s *Scope) Clone() *Scope {
	return &Scope{tags: s.GetTags()}
}

// Clear removes all tags from the scope
func (s *Scope) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags = make(map[string]string)
}

// Global default scope
var defaultScope = NewScope()

// TransactionContext holds transaction data including segments
type TransactionContext struct {
	Id       string
	Segments []Segment
	mu       sync.Mutex
}

// AddSegment adds a segment to the transaction context
func (t *TransactionContext) AddSegment(seg Segment) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Segments = append(t.Segments, seg)
}

// GetSegments returns a copy of segments
func (t *TransactionContext) GetSegments() []Segment {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Segments
}

// ConfigureScope modifies the global default scope (persistent changes)
func ConfigureScope(fn func(*Scope)) {
	fn(defaultScope)
}

// WithScope creates an isolated scope clone for the callback and returns a new context
func WithScope(ctx context.Context, fn func(*Scope)) context.Context {
	scope := GetScopeFromContext(ctx).Clone()
	fn(scope)
	return context.WithValue(ctx, CtxScopeKey, scope)
}

// GetScopeFromContext retrieves scope from context, falls back to default clone
func GetScopeFromContext(ctx context.Context) *Scope {
	if ctx == nil {
		return defaultScope.Clone()
	}
	if scope, ok := ctx.Value(string(CtxScopeKey)).(*Scope); ok && scope != nil {
		return scope
	}

	return defaultScope.Clone()
}

// GetScopeFromGin is a convenience helper for Gin handlers
func GetScopeFromGin(c *gin.Context) *Scope {
	if scope, ok := c.Get(string(CtxScopeKey)); ok {
		if s, ok := scope.(*Scope); ok {
			return s
		}
	}
	return GetScopeFromContext(c.Request.Context())
}

// GetTransactionFromContext retrieves the transaction context
func GetTransactionFromContext(ctx context.Context) *TransactionContext {
	if val, ok := ctx.Value(string(CtxTransactionKey)).(*TransactionContext); ok {
		return val
	}
	return nil
}

func GetTransactionIdFromContext(ctx context.Context) *string {
	if txn := GetTransactionFromContext(ctx); txn != nil {
		return &txn.Id
	}
	return nil
}

// GetHostname returns the hostname, cached for efficiency
var cachedHostname string
var hostnameOnce sync.Once

func getHostname() string {
	hostnameOnce.Do(func() {
		hostname, err := os.Hostname()
		if err != nil {
			cachedHostname = "unknown"
		} else {
			cachedHostname = hostname
		}
	})
	return cachedHostname
}

// GetEnvironment returns the environment from TRACEWAY_ENV or GO_ENV
var cachedEnvironment string
var envOnce sync.Once

func getEnvironment() string {
	envOnce.Do(func() {
		cachedEnvironment = os.Getenv("TRACEWAY_ENV")
		if cachedEnvironment == "" {
			cachedEnvironment = os.Getenv("GO_ENV")
		}
		if cachedEnvironment == "" {
			cachedEnvironment = "development"
		}
	})
	return cachedEnvironment
}

func CaptureStack(skip int) []runtime.Frame {
	const maxDepth = 64
	pcs := make([]uintptr, maxDepth)

	// +2 skips runtime.Callers and CaptureStack
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	result := make([]runtime.Frame, 0, n)

	for {
		frame, more := frames.Next()
		result = append(result, frame)
		if !more {
			break
		}
	}
	return result
}

func FormatRWithStack(r interface{}, frames []runtime.Frame) string {
	var err error
	switch v := r.(type) {
	case error:
		err = v
	default:
		err = fmt.Errorf("%v", v)
	}
	return FormatErrorWithStack(err, frames)
}

func FormatErrorWithStack(err error, frames []runtime.Frame) string {
	var sb strings.Builder

	if err != nil {
		errType := reflect.TypeOf(err).String()
		fmt.Fprintf(&sb, "%s: %s\n", errType, err.Error())
	}

	for _, frame := range frames {
		fn := frame.Function
		// Extract just the function/method name
		if idx := strings.LastIndex(fn, "/"); idx >= 0 {
			fn = fn[idx+1:]
		}
		if idx := strings.Index(fn, "."); idx >= 0 {
			fn = fn[idx+1:] // Remove package name, keep (*Type).Method
		}
		fmt.Fprintf(&sb, "%s()\n", fn)
		fmt.Fprintf(&sb, "    %s:%d\n", frame.File, frame.Line)
	}

	return sb.String()
}

type ExceptionStackTrace struct {
	TransactionId *string           `json:"transactionId"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    time.Time         `json:"recordedAt"`
	Scope         map[string]string `json:"scope,omitempty"`
	IsMessage     bool              `json:"isMessage"`
}

type MetricRecord struct {
	Name       string    `json:"name"`
	Value      float64   `json:"value"`
	RecordedAt time.Time `json:"recordedAt"`
}

type Transaction struct {
	Id         string            `json:"id"`
	Endpoint   string            `json:"endpoint"`
	Duration   time.Duration     `json:"duration"`
	RecordedAt time.Time         `json:"recordedAt"`
	StatusCode int               `json:"statusCode"`
	BodySize   int               `json:"bodySize"`
	ClientIP   string            `json:"clientIP"`
	Scope      map[string]string `json:"scope,omitempty"`
	Segments   []Segment         `json:"segments,omitempty"`
}

// Segment represents a timed slice within a transaction
type Segment struct {
	Id        string        `json:"id"`
	Name      string        `json:"name"`
	StartTime time.Time     `json:"startTime"`
	Duration  time.Duration `json:"duration"`
}

// ActiveSegment is a running segment that can be ended
type ActiveSegment struct {
	segment   Segment
	txn       *TransactionContext
	startedAt time.Time
	ended     bool
	mu        sync.Mutex
}

// End completes the segment and records its duration
func (s *ActiveSegment) End() {
	if s == nil || s.txn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ended {
		return // Already ended
	}

	s.ended = true
	s.segment.Duration = time.Since(s.startedAt)
	s.txn.AddSegment(s.segment)
}

type CollectionFrame struct {
	StackTraces  []*ExceptionStackTrace `json:"stackTraces"`
	Metrics      []*MetricRecord        `json:"metrics"`
	Transactions []*Transaction         `json:"transactions"`
}

type collectionFrameMessageType int

const (
	CollectionFrameMessageTypeException   = 0
	CollectionFrameMessageTypeMetric      = 1
	CollectionFrameMessageTypeTransaction = 2
	CollectionFrameMessageTypeClearFrames = 3
)

type CollectionFrameMessage struct {
	msgType                  collectionFrameMessageType
	exceptionStackTrace      *ExceptionStackTrace
	metric                   *MetricRecord
	transaction              *Transaction
	collectionFramesToRemove []*CollectionFrame
}

type CollectionFrameStore struct {
	current      *CollectionFrame
	currentSetAt time.Time
	sendQueue    TypedRing[*CollectionFrame, CollectionFrame]

	mu sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup

	messageQueue chan CollectionFrameMessage

	lastUploadStarted *time.Time

	// Config fields
	apiUrl              string
	token               string
	debug               bool
	maxCollectionFrames int
	collectionInterval  time.Duration
	uploadTimeout       time.Duration
	metricsInterval     time.Duration
	version             string
	serverName          string
}

func InitCollectionFrameStore(
	apiUrl string,
	token string,
	debug bool,
	maxCollectionFrames int,
	collectionInterval time.Duration,
	uploadTimeout time.Duration,
	metricsInterval time.Duration,
	version string,
	serverName string,
) *CollectionFrameStore {
	store := &CollectionFrameStore{
		current:      nil,
		currentSetAt: time.Now(),
		sendQueue:    InitTypedRing[*CollectionFrame](maxCollectionFrames),
		stopCh:       make(chan struct{}),
		messageQueue: make(chan CollectionFrameMessage),

		apiUrl: apiUrl,
		token:  token,
		debug:  debug,

		maxCollectionFrames: maxCollectionFrames,
		collectionInterval:  collectionInterval,
		uploadTimeout:       uploadTimeout,
		metricsInterval:     metricsInterval,
		version:             version,
		serverName:          serverName,
	}

	store.wg.Add(1)
	go store.process()
	go store.processMetrics()

	return store
}

const (
	MetricNameMemoryUsage  = "mem.used"
	MetricNameMemoryTotal  = "mem.total"
	MetricNameCpuUsage     = "cpu.used_pcnt"
	MetricNameGoRoutines   = "go.go_routines"
	MetricNameHeapObjects  = "go.heap_objects"
	MetricNameNumGC        = "go.num_gc"
	MetricNameGCPauseTotal = "go.gc_pause"
)

func (s *CollectionFrameStore) processMetrics() {
	metricsTicker := time.NewTicker(s.metricsInterval)
	defer metricsTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-metricsTicker.C:
			s.safeProcessMetrics()
		}
	}
}

func (s *CollectionFrameStore) safeProcessMetrics() {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Traceway: failed to get metrics data")
		}
	}()
	cpuPercent, err := cpu.GetCpuPercent(time.Second)
	if err == nil {
		CaptureMetric(MetricNameCpuUsage, cpuPercent)
	} else {
		if s.debug {
			log.Println("Traceway cpu not read", err)
		}
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	CaptureMetric(MetricNameMemoryUsage, float64(m.Alloc)/1024/1024)

	totalMem, err := mem.GetTotalMemory()
	if err == nil && totalMem > 0 {
		// Convert bytes to MB for total system memory
		CaptureMetric(MetricNameMemoryTotal, float64(totalMem)/1024/1024)
	} else {
		if s.debug {
			log.Println("Traceway total mem not read", err)
		}
	}
	CaptureMetric(MetricNameGoRoutines, float64(runtime.NumGoroutine()))
	CaptureMetric(MetricNameHeapObjects, float64(m.HeapObjects))
	CaptureMetric(MetricNameNumGC, float64(m.NumGC))
	CaptureMetric(MetricNameGCPauseTotal, float64(m.PauseTotalNs))
}
func (s *CollectionFrameStore) process() {
	defer s.wg.Done()

	rotationTicker := time.NewTicker(s.collectionInterval)
	defer rotationTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-rotationTicker.C:
			if s.current != nil {
				if s.currentSetAt.Before(time.Now().Add(-s.collectionInterval)) {
					s.rotateCurrentCollectionFrame()
					s.processSendQueue()
				}
			} else if s.sendQueue.len > 0 {
				s.processSendQueue()
			}
		case msg := <-s.messageQueue:
			if msg.msgType == CollectionFrameMessageTypeClearFrames {
				s.sendQueue.Remove(msg.collectionFramesToRemove)
				continue
			}

			if s.current == nil {
				// we need to start a new frame
				s.current = &CollectionFrame{}
				s.currentSetAt = time.Now()
			}

			switch msg.msgType {
			case CollectionFrameMessageTypeException:
				s.current.StackTraces = append(s.current.StackTraces, msg.exceptionStackTrace)
			case CollectionFrameMessageTypeMetric:
				s.current.Metrics = append(s.current.Metrics, msg.metric)
			case CollectionFrameMessageTypeTransaction:
				s.current.Transactions = append(s.current.Transactions, msg.transaction)
			}
		}
	}
}
func (s *CollectionFrameStore) rotateCurrentCollectionFrame() {
	s.sendQueue.Push(s.current)
	s.current = nil
}
func (s *CollectionFrameStore) processSendQueue() {
	// we are triggering an upload - we need to make sure no other uploads are going on
	if s.lastUploadStarted == nil || s.lastUploadStarted.Before(time.Now().Add(s.uploadTimeout)) {
		now := time.Now()
		s.lastUploadStarted = &now
		go s.triggerUpload(s.sendQueue.ReadAll())
	}
}

// Report adds an exception event to the current envelope
func (s *CollectionFrameStore) triggerUpload(framesToSend []*CollectionFrame) {
	defer func() {
		if r := recover(); s.debug && r != nil {
			log.Print("Traceway: failed to upload the CollectionFrame")
			log.Println(r)
			debug.PrintStack()
		}
	}()

	jsonData, err := json.Marshal(gin.H{
		"collectionFrames": framesToSend,
		"appVersion":       s.version,
		"serverName":       s.serverName,
	})
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to marshal frames: %v", err)
		}
		return
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		if s.debug {
			log.Printf("Traceway: gz write failed: %v", err)
		}
		return
	}
	if err := gz.Close(); err != nil { // Close flushes the compressed data
		if s.debug {
			log.Printf("Traceway: gz write failed: %v", err)
		}
		return
	}

	req, err := http.NewRequest("POST", s.apiUrl, &buf)
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to create request: %v", err)
		}
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to send request: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// we need to clear out the frames
		s.messageQueue <- CollectionFrameMessage{
			msgType:                  CollectionFrameMessageTypeClearFrames,
			collectionFramesToRemove: framesToSend,
		}
	}
}

var collectionFrameStore *CollectionFrameStore

func PrintCollectionFrameMetrics() {
	val, _ := json.Marshal(collectionFrameStore.current)
	fmt.Println("current", string(val))

	fmt.Println("currentSetAt", collectionFrameStore.currentSetAt)
	fmt.Println("sendQueue", collectionFrameStore.sendQueue)
	fmt.Println("lastUploadStarted", collectionFrameStore.lastUploadStarted)
	fmt.Println("apiUrl", collectionFrameStore.apiUrl)
	fmt.Println("token", collectionFrameStore.token)
	fmt.Println("debug", collectionFrameStore.debug)
	fmt.Println("maxCollectionFrames", collectionFrameStore.maxCollectionFrames)
	fmt.Println("collectionInterval", collectionFrameStore.collectionInterval)
	fmt.Println("uploadTimeout", collectionFrameStore.uploadTimeout)
}

type TracewayOptions struct {
	debug               bool
	maxCollectionFrames int
	collectionInterval  time.Duration
	uploadTimeout       time.Duration
	metricsInterval     time.Duration
	version             string
	serverName          string
}

func NewTracewayOptions(options ...func(*TracewayOptions)) *TracewayOptions {
	svr := &TracewayOptions{
		maxCollectionFrames: 12,
		collectionInterval:  5 * time.Second,
		uploadTimeout:       2 * time.Second,
		metricsInterval:     30 * time.Second,
		version:             "",
		serverName:          getHostname(),
	}
	for _, o := range options {
		o(svr)
	}
	return svr
}
func WithDebug(val bool) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.debug = val
	}
}
func WithMaxCollectionFrames(val int) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.maxCollectionFrames = val
	}
}
func WithCollectionInterval(val time.Duration) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.collectionInterval = val
	}
}
func WithUploadTimeout(val time.Duration) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.uploadTimeout = val
	}
}
func WithMetricsInterval(val time.Duration) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.metricsInterval = val
	}
}
func WithVersion(val string) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.version = val
	}
}
func WithServerName(val string) func(*TracewayOptions) {
	return func(s *TracewayOptions) {
		s.serverName = val
	}
}

func Init(connectionString string, options ...func(*TracewayOptions)) error {
	if collectionFrameStore != nil {
		return fmt.Errorf("Second Traceway initialization detected")
	}
	connParts := strings.Split(connectionString, "@")

	token := connParts[0]
	apiUrl := connParts[1]

	tracewayOptions := NewTracewayOptions(options...)

	collectionFrameStore = InitCollectionFrameStore(
		apiUrl,
		token,
		tracewayOptions.debug,
		tracewayOptions.maxCollectionFrames,
		tracewayOptions.collectionInterval,
		tracewayOptions.uploadTimeout,
		tracewayOptions.metricsInterval,
		tracewayOptions.version,
		tracewayOptions.serverName,
	)
	return nil
}

func CaptureMetric(name string, value float64) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeMetric,
		metric: &MetricRecord{
			Name:       name,
			Value:      value,
			RecordedAt: time.Now(),
		},
	}
}

// CaptureTransaction captures a transaction without scope (backward compatible)
func CaptureTransaction(
	txn *TransactionContext,
	endpoint string,
	d time.Duration,
	startedAt time.Time,
	statusCode, bodySize int,
	clientIP string,
) {
	CaptureTransactionWithScope(txn, endpoint, d, startedAt, statusCode, bodySize, clientIP, nil)
}

// CaptureTransactionWithScope captures a transaction with scope
func CaptureTransactionWithScope(
	txn *TransactionContext,
	endpoint string,
	d time.Duration,
	startedAt time.Time,
	statusCode, bodySize int,
	clientIP string,
	scope map[string]string,
) {
	if txn == nil {
		return
	}
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeTransaction,
		transaction: &Transaction{
			Id:         txn.Id,
			Endpoint:   endpoint,
			Duration:   d,
			RecordedAt: startedAt,
			StatusCode: statusCode,
			BodySize:   bodySize,
			ClientIP:   clientIP,
			Scope:      scope,
			Segments:   txn.GetSegments(),
		},
	}
}

// StartSegment starts a segment using transaction ID from context
func StartSegment(ctx context.Context, name string) *ActiveSegment {
	txn := GetTransactionFromContext(ctx)
	if txn == nil {
		return nil
	}
	now := time.Now()
	return &ActiveSegment{
		segment: Segment{
			Id:        uuid.NewString(),
			Name:      name,
			StartTime: now,
		},
		txn:       txn,
		startedAt: now,
	}
}

// CaptureTransactionException captures an exception linked to a transaction (backward compatible)
func CaptureTransactionException(transactionId string, stacktrace string) {
	CaptureTransactionExceptionWithScope(transactionId, stacktrace, nil)
}

// CaptureTransactionExceptionWithScope captures an exception linked to a transaction with scope
func CaptureTransactionExceptionWithScope(transactionId string, stacktrace string, scope map[string]string) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			TransactionId: &transactionId,
			StackTrace:    stacktrace,
			RecordedAt:    time.Now(),
			Scope:         scope,
		},
	}
}

// CaptureException captures an exception without context (backward compatible)
func CaptureException(err error) {
	CaptureExceptionWithScope(err, nil, nil)
}

// CaptureExceptionWithScope captures an exception with scope
func CaptureExceptionWithScope(err error, scope map[string]string, transactionId *string) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			TransactionId: transactionId,
			StackTrace:    FormatErrorWithStack(err, CaptureStack(2)),
			RecordedAt:    time.Now(),
			Scope:         scope,
		},
	}
}

// CaptureExceptionWithContext captures an exception extracting scope from context
func CaptureExceptionWithContext(ctx context.Context, err error) {
	scope := GetScopeFromContext(ctx)
	CaptureExceptionWithScope(err, scope.GetTags(), GetTransactionIdFromContext(ctx))
}

// Recover recovers from panic and captures it (backward compatible, no scope)
func Recover() {
	r := recover()

	if r != nil {
		collectionFrameStore.messageQueue <- CollectionFrameMessage{
			msgType: CollectionFrameMessageTypeException,
			exceptionStackTrace: &ExceptionStackTrace{
				TransactionId: nil,
				StackTrace:    FormatRWithStack(r, CaptureStack(2)),
				RecordedAt:    time.Now(),
			},
		}
	}
}

// RecoverWithContext recovers from panic and captures it with scope from context
func RecoverWithContext(ctx context.Context) {
	r := recover()

	if r != nil {
		scope := GetScopeFromContext(ctx)
		collectionFrameStore.messageQueue <- CollectionFrameMessage{
			msgType: CollectionFrameMessageTypeException,
			exceptionStackTrace: &ExceptionStackTrace{
				TransactionId: nil,
				StackTrace:    FormatRWithStack(r, CaptureStack(2)),
				RecordedAt:    time.Now(),
				Scope:         scope.GetTags(),
			},
		}
	}
}

// CaptureMessage captures a message as an exception with minimal stack trace
func CaptureMessage(msg string) {
	CaptureMessageWithContext(context.Background(), msg)
}

// CaptureMessageWithContext captures a message with context as an exception
func CaptureMessageWithContext(ctx context.Context, msg string) {
	if collectionFrameStore == nil {
		return
	}

	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}

	stackTrace := fmt.Sprintf("Message: %s\n    at %s:%d", msg, file, line)

	scope := GetScopeFromContext(ctx)
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			TransactionId: GetTransactionIdFromContext(ctx),
			StackTrace:    stackTrace,
			RecordedAt:    time.Now(),
			Scope:         scope.GetTags(),
			IsMessage:     true,
		},
	}
}
