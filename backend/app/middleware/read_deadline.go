package middleware

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const minThroughputBytesPerSec = 1 << 10

var throughputGracePeriod = 20 * time.Second

var ErrBodyTooSlow = errors.New("request body delivered too slowly")

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
	deadline time.Time
	lastSet  time.Time
	start    time.Time
	read     int64
	done     bool
}

func (p *progressBody) Read(b []byte) (int, error) {
	n, err := p.rc.Read(b)
	if p.done {
		return n, err
	}
	p.read += int64(n)
	if err == io.EOF || (p.declared >= 0 && p.read >= p.declared) {
		p.done = true
		_ = p.ctl.SetReadDeadline(time.Time{})
		return n, err
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

func (p *progressBody) Close() error { return p.rc.Close() }

func GuardBodyRead(c *gin.Context, idle, total time.Duration) {
	if c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
		return
	}
	now := time.Now()
	if existing, ok := c.Get(bodyGuardContextKey); ok {
		if p, ok := existing.(*progressBody); ok {
			if p.done {
				return
			}
			p.idle = idle
			p.deadline = now.Add(total)
			p.arm(now)
			return
		}
	}

	ctl := http.NewResponseController(c.Writer)
	p := &progressBody{
		rc:       c.Request.Body,
		ctl:      ctl,
		declared: c.Request.ContentLength,
		idle:     idle,
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
