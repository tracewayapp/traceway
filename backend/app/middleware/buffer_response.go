package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"maps"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

var errHijackUnderTransaction = fmt.Errorf("hijack is not supported on a transactional route: %w", http.ErrNotSupported)

// responseBuffer holds a handler's status and body until Transactional knows
// whether the transaction committed.
type responseBuffer struct {
	gin.ResponseWriter
	ctx     *gin.Context
	body    bytes.Buffer
	status  int
	written bool
	headers http.Header
}

func bufferResponse(c *gin.Context) *responseBuffer {
	buf := &responseBuffer{
		ResponseWriter: c.Writer,
		ctx:            c,
		status:         c.Writer.Status(),
		headers:        c.Writer.Header().Clone(),
	}
	c.Writer = buf
	return buf
}

func (b *responseBuffer) WriteHeader(code int) {
	if code <= 0 || b.written {
		return
	}
	b.status = code
}

// WriteHeaderNow locks the status the way gin's writer does; nothing reaches
// the wire until release.
func (b *responseBuffer) WriteHeaderNow() {
	b.written = true
}

func (b *responseBuffer) Flush() {
	b.WriteHeaderNow()
}

func (b *responseBuffer) Write(data []byte) (int, error) {
	b.WriteHeaderNow()
	return b.body.Write(data)
}

func (b *responseBuffer) WriteString(s string) (int, error) {
	b.WriteHeaderNow()
	return b.body.WriteString(s)
}

func (b *responseBuffer) Status() int {
	return b.status
}

func (b *responseBuffer) Size() int {
	if !b.written {
		return -1
	}
	return b.body.Len()
}

func (b *responseBuffer) Written() bool {
	return b.written
}

func (b *responseBuffer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errHijackUnderTransaction
}

// Unwrap keeps http.ResponseController reaching the connection from inside a
// handler (SetWriteDeadline and friends), as it does through gin's own writer.
func (b *responseBuffer) Unwrap() http.ResponseWriter {
	return b.ResponseWriter
}

// discard drops the buffered response and restores the headers as they were
// before the handler ran, so a Content-Type, Location or Set-Cookie meant for
// the discarded response does not ride out on the 500 that replaces it.
func (b *responseBuffer) discard() {
	clear(b.Header())
	maps.Copy(b.Header(), b.headers)
	b.ctx.Writer = b.ResponseWriter
}

func (b *responseBuffer) release() {
	b.ctx.Writer = b.ResponseWriter
	b.ResponseWriter.WriteHeader(b.status)
	b.ResponseWriter.Write(b.body.Bytes())
}
