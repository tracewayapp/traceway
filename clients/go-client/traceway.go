package traceway

import (
	"bytes"
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
)

const (
	MaxCollectionFrames = 5
	CollectionInterval  = time.Minute
	UploadTimeout       = 2 * time.Second
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
	transactionId *string
	stackTrace    string
	recordedAt    time.Time
}

type MetricsRecord struct {
	name       string
	value      float32
	recordedAt time.Time
}

type Transaction struct {
	id         string
	endpoint   string
	duration   time.Duration
	recordedAt time.Time
}

type CollectionFrame struct {
	stackTraces  []*ExceptionStackTrace
	metrics      []*MetricsRecord
	transactions []*Transaction
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
	droppedCount int64

	mu sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup

	messageQueue chan CollectionFrameMessage

	lastUploadStarted *time.Time

	// Config fields
	apiUrl string
	token  string
	debug  bool
}

func InitCollectionFrameStore() *CollectionFrameStore {
	store := &CollectionFrameStore{
		current:      nil,
		currentSetAt: time.Now(),
		sendQueue:    InitTypedRing[*CollectionFrame](5),
		stopCh:       make(chan struct{}),
		messageQueue: make(chan CollectionFrameMessage),
	}

	store.wg.Add(1)
	go store.process()

	return store
}

func (s *CollectionFrameStore) process() {
	defer s.wg.Done()

	rotationTicker := time.NewTicker(CollectionInterval)
	defer rotationTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-rotationTicker.C:
			if s.current != nil {
				if s.currentSetAt.Before(time.Now().Add(-CollectionInterval)) {
					s.rotateCurrentCollectionFrame()
					s.processSendQueue()
				}
			} else if s.sendQueue.len > 0 {
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
				s.current.stackTraces = append(s.current.stackTraces, msg.exceptionStackTrace)
			case CollectionFrameMessageTypeMetric:
				s.current.metrics = append(s.current.metrics, msg.metric)
			case CollectionFrameMessageTypeTransaction:
				s.current.transactions = append(s.current.transactions, msg.transaction)
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
	if s.lastUploadStarted == nil || s.lastUploadStarted.Before(time.Now().Add(UploadTimeout)) {
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

	jsonData, err := json.Marshal(framesToSend)
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to marshal frames: %v", err)
		}
		return
	}

	req, err := http.NewRequest("POST", s.apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to create request: %v", err)
		}
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if s.debug {
			log.Printf("Traceway: failed to send request: %v", err)
		}
		return
	}
	defer resp.Body.Close()
}

var collectionFrameStore = InitCollectionFrameStore()

func CaptureMetric(name string, value float32) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeMetric,
		metric: &MetricsRecord{
			name:       name,
			value:      value,
			recordedAt: time.Now(),
		},
	}
}
func CaptureException(err error) {
	collectionFrameStore.messageQueue <- CollectionFrameMessage{
		msgType: CollectionFrameMessageTypeException,
		exceptionStackTrace: &ExceptionStackTrace{
			transactionId: nil, // for regular recover we don't need a transaction
			stackTrace:    FormatErrorWithStack(err, CaptureStack(2)),
			recordedAt:    time.Now(),
		},
	}
}

func Recover() {
	r := recover()

	if r != nil {
		var err error
		switch v := r.(type) {
		case error:
			err = v
		default:
			err = fmt.Errorf("%v", v)
		}
		collectionFrameStore.messageQueue <- CollectionFrameMessage{
			msgType: CollectionFrameMessageTypeException,
			exceptionStackTrace: &ExceptionStackTrace{
				transactionId: nil, // for regular recover we don't need a transaction
				stackTrace:    FormatErrorWithStack(err, CaptureStack(2)),
				recordedAt:    time.Now(),
			},
		}
	}
}
