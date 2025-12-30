package traceway

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

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
	TransactionId *string   `json:"transactionId"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    time.Time `json:"recordedAt"`
}

type MetricsRecord struct {
	Name       string    `json:"name"`
	Value      float32   `json:"value"`
	RecordedAt time.Time `json:"recordedAt"`
}

type Transaction struct {
	Id         string        `json:"id"`
	Endpoint   string        `json:"endpoint"`
	Duration   time.Duration `json:"duration"`
	RecordedAt time.Time     `json:"recordedAt"`
	StatusCode int           `json:"statusCode"`
	BodySize   int           `json:"bodySize"`
	ClientIP   string        `json:"clientIP"`
}

type CollectionFrame struct {
	StackTraces  []*ExceptionStackTrace `json:"stackTraces"`
	Metrics      []*MetricsRecord       `json:"metrics"`
	Transactions []*Transaction         `json:"transactions"`
}

type collectionFrameMessageType int

const (
	CollectionFrameMessageTypeException   = 0
	CollectionFrameMessageTypeMetric      = 1
	CollectionFrameMessageTypeTransaction = 2
)

type CollectionFrameMessage struct {
	msgType             collectionFrameMessageType
	exceptionStackTrace *ExceptionStackTrace
	metric              *MetricsRecord
	transaction         *Transaction
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
	app                 string
	apiUrl              string
	token               string
	debug               bool
	maxCollectionFrames int
	collectionInterval  time.Duration
	uploadTimeout       time.Duration
}

func InitCollectionFrameStore(
	app string,
	apiUrl string,
	token string,
	debug bool,
	maxCollectionFrames int,
	collectionInterval time.Duration,
	uploadTimeout time.Duration,
) *CollectionFrameStore {
	store := &CollectionFrameStore{
		current:      nil,
		currentSetAt: time.Now(),
		sendQueue:    InitTypedRing[*CollectionFrame](maxCollectionFrames),
		stopCh:       make(chan struct{}),
		messageQueue: make(chan CollectionFrameMessage),

		app:    app,
		apiUrl: apiUrl,
		token:  token,
		debug:  debug,

		maxCollectionFrames: maxCollectionFrames,
		collectionInterval:  collectionInterval,
		uploadTimeout:       uploadTimeout,
	}

	store.wg.Add(1)
	go store.process()

	return store
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
			fmt.Println("TRIGGERING THE rotationTicker.C")
			if s.current != nil {
				if s.currentSetAt.Before(time.Now().Add(-s.collectionInterval)) {
					fmt.Println("ROTATING AND PROCESSING")
					s.rotateCurrentCollectionFrame()
					s.processSendQueue()
				}
			} else if s.sendQueue.len > 0 {
				fmt.Println("PROCESSING SEND QUEUE")
				s.processSendQueue()
			}
		case msg := <-s.messageQueue:
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
	fmt.Println("processSendQueue")
	// we are triggering an upload - we need to make sure no other uploads are going on
	if s.lastUploadStarted == nil || s.lastUploadStarted.Before(time.Now().Add(s.uploadTimeout)) {
		fmt.Println("ABOUT TO TRIGGER THE UPLOAD")
		now := time.Now()
		s.lastUploadStarted = &now
		go s.triggerUpload(s.sendQueue.ReadAll())
	}
}

// Report adds an exception event to the current envelope
func (s *CollectionFrameStore) triggerUpload(framesToSend []*CollectionFrame) {
	fmt.Println("TRIGGER UPLOAD STARTED")
	defer func() {
		if r := recover(); s.debug && r != nil {
			log.Print("Traceway: failed to upload the CollectionFrame")
			log.Println(r)
			debug.PrintStack()
		}
	}()

	jsonData, err := json.Marshal(gin.H{
		"app":    s.app,
		"frames": framesToSend,
	})
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to marshal frames: %v", err)
		}
		return
	}

	fmt.Println("SENDING POST TO ", s.apiUrl, string(jsonData))

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

	fmt.Println("RESPONDED WITH", resp.StatusCode)
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
}

func NewTracewayOptions(options ...func(*TracewayOptions)) *TracewayOptions {
	svr := &TracewayOptions{
		maxCollectionFrames: 12,
		collectionInterval:  5 * time.Second,
		uploadTimeout:       2 * time.Second,
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

func Init(app, connectionString string, options ...func(*TracewayOptions)) error {
	if collectionFrameStore != nil {
		return fmt.Errorf("Second Traceway initialization detected")
	}
	connParts := strings.Split(connectionString, "@")

	token := connParts[0]
	apiUrl := connParts[1]

	tracewayOptions := NewTracewayOptions(options...)

	collectionFrameStore = InitCollectionFrameStore(
		app,
		apiUrl,
		token,
		tracewayOptions.debug,
		tracewayOptions.maxCollectionFrames,
		tracewayOptions.collectionInterval,
		tracewayOptions.uploadTimeout,
	)
	return nil
}

func CaptureMetric(name string, value float32) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeMetric,
		metric: &MetricsRecord{
			Name:       name,
			Value:      value,
			RecordedAt: time.Now(),
		},
	}
}

func CaptureTransaction(
	transactionId string,
	endpoint string,
	d time.Duration,
	startedAt time.Time,
	statusCode, bodySize int,
	clientIP string,
) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeTransaction,
		transaction: &Transaction{
			Id:         transactionId, // for regular recover we don't need a transaction
			Endpoint:   endpoint,
			Duration:   d,
			RecordedAt: startedAt,
			StatusCode: statusCode,
			BodySize:   bodySize,
			ClientIP:   clientIP,
		},
	}
}
func CaptureTransactionException(transactionId string, stacktrace string) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			TransactionId: &transactionId,
			StackTrace:    stacktrace,
			RecordedAt:    time.Now(),
		},
	}
}
func CaptureException(err error) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			TransactionId: nil, // for regular recover we don't need a transaction
			StackTrace:    FormatErrorWithStack(err, CaptureStack(2)),
			RecordedAt:    time.Now(),
		},
	}
}

func Recover() {
	r := recover()

	if r != nil {
		collectionFrameStore.messageQueue <- CollectionFrameMessage{
			msgType: CollectionFrameMessageTypeException,
			exceptionStackTrace: &ExceptionStackTrace{
				TransactionId: nil, // for regular recover we don't need a transaction
				StackTrace:    FormatRWithStack(r, CaptureStack(2)),
				RecordedAt:    time.Now(),
			},
		}
	}
}
