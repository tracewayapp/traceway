package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

const minThroughputBytesPerSec = 1 << 10

var throughputGracePeriod = 20 * time.Second

var ErrBodyTooSlow = errors.New("request body delivered too slowly")

// The raw deadline error is a *net.OpError naming the listener address, and
// handlers echo bind errors verbatim, so it is replaced before it leaves Read.
var ErrBodyTimedOut = fmt.Errorf("request body timed out: %w", os.ErrDeadlineExceeded)

const (
	DefaultBodyIdle  = 30 * time.Second
	DefaultBodyTotal = 10 * time.Minute
)

const bodyGuardContextKey = "bodyGuard"

type progressBody struct {
	rc       io.ReadCloser
	ctl      *http.ResponseController
	declared int64
	idle     time.Duration
	total    time.Duration
	deadline time.Time
	lastSet  time.Time
	start    time.Time
	read     int64
	started  bool
	done     bool
	cutOff   bool
}

func (p *progressBody) Read(b []byte) (int, error) {
	// The clock starts at the first read: everything before it (auth, a
	// queued main-DB transaction) is the server's time, not the client's.
	if !p.started {
		p.started = true
		p.restart(time.Now(), p.idle, p.total)
	}
	n, err := p.rc.Read(b)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		err = ErrBodyTimedOut
	}
	if p.done {
		return n, err
	}
	p.read += int64(n)
	if err == io.EOF || (p.declared >= 0 && p.read >= p.declared) {
		p.done = true
		_ = p.ctl.SetReadDeadline(time.Time{})
		return n, err
	}
	if err != nil && !errors.Is(err, ErrBodyTimedOut) {
		p.cutOff = true
	}
	if n == 0 {
		return n, err
	}
	now := time.Now()
	if elapsed := now.Sub(p.start); elapsed > throughputGracePeriod && float64(p.read)/elapsed.Seconds() < minThroughputBytesPerSec {
		return n, ErrBodyTooSlow
	}
	if now.Sub(p.lastSet) >= p.idle/4 {
		p.arm(now)
	}
	return n, err
}

func (p *progressBody) arm(now time.Time) {
	next := now.Add(p.idle)
	if next.After(p.deadline) {
		next = p.deadline
	}
	_ = p.ctl.SetReadDeadline(next)
	p.lastSet = now
}

// restart re-arms the deadlines and restarts the throughput clock, so time a
// request spent parked before its handler read anything (an admission gate)
// is not held against it.
func (p *progressBody) restart(now time.Time, idle, total time.Duration) {
	p.idle = idle
	p.total = total
	p.deadline = now.Add(total)
	p.start = now
	p.read = 0
	p.arm(now)
}

func (p *progressBody) Close() error { return p.rc.Close() }

// bodyCutOff reports whether the connection failed before the body was complete: the client hung up or was cut off
// mid-upload (a phone losing signal, a closed tab), which leaves a decoder with a truncated body like a malformed one.
func bodyCutOff(c *gin.Context) bool {
	if existing, ok := c.Get(bodyGuardContextKey); ok {
		if p, ok := existing.(*progressBody); ok {
			return p.cutOff
		}
	}
	return false
}

func restartBodyGuard(c *gin.Context) {
	if existing, ok := c.Get(bodyGuardContextKey); ok {
		if p, ok := existing.(*progressBody); ok && !p.done {
			p.restart(time.Now(), p.idle, p.total)
		}
	}
}

func GuardBodyRead(c *gin.Context, idle, total time.Duration) {
	if c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
		// Nothing to read, but the server's ReadTimeout is still armed on the
		// connection and would cancel a long-lived handler such as the MCP GET
		// stream, so lift it the way a completed body does.
		_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
		return
	}
	now := time.Now()
	if existing, ok := c.Get(bodyGuardContextKey); ok {
		if p, ok := existing.(*progressBody); ok {
			if p.done {
				return
			}
			p.restart(now, idle, total)
			return
		}
	}

	ctl := http.NewResponseController(c.Writer)
	p := &progressBody{
		rc:       c.Request.Body,
		ctl:      ctl,
		declared: c.Request.ContentLength,
		idle:     idle,
		total:    total,
		deadline: now.Add(total),
		start:    now,
	}
	if err := ctl.SetReadDeadline(now.Add(min(idle, total))); err != nil {
		return
	}
	p.lastSet = now
	c.Request.Body = p
	c.Set(bodyGuardContextKey, p)
}

func GuardBodyReads(idle, total time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		GuardBodyRead(c, idle, total)
		c.Next()
	}
}
