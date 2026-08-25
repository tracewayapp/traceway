package middleware

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const minThroughputBytesPerSec = 4 << 10

var throughputGracePeriod = 20 * time.Second

var ErrBodyTooSlow = errors.New("request body delivered too slowly")

type progressBody struct {
	rc       io.ReadCloser
	ctl      *http.ResponseController
	idle     time.Duration
	deadline time.Time
	lastSet  time.Time
	start    time.Time
	read     int64
}

func (p *progressBody) Read(b []byte) (int, error) {
	n, err := p.rc.Read(b)
	if n > 0 {
		p.read += int64(n)
		now := time.Now()

		if elapsed := now.Sub(p.start); elapsed > throughputGracePeriod {
			if float64(p.read)/elapsed.Seconds() < minThroughputBytesPerSec {
				return n, ErrBodyTooSlow
			}
		}

		if now.Sub(p.lastSet) >= p.idle/4 {
			next := now.Add(p.idle)
			if next.After(p.deadline) {
				next = p.deadline
			}
			_ = p.ctl.SetReadDeadline(next)
			p.lastSet = now
		}
	}
	return n, err
}

func (p *progressBody) Close() error { return p.rc.Close() }

func GuardBodyRead(c *gin.Context, idle, total time.Duration) {
	if c.Request == nil || c.Request.Body == nil {
		return
	}
	ctl := http.NewResponseController(c.Writer)
	now := time.Now()
	deadline := now.Add(total)

	first := now.Add(idle)
	if first.After(deadline) {
		first = deadline
	}
	if err := ctl.SetReadDeadline(first); err != nil {
		return
	}
	c.Request.Body = &progressBody{rc: c.Request.Body, ctl: ctl, idle: idle, deadline: deadline, lastSet: now, start: now}
}
