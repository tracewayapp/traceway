package otelcontrollers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxBodySize = 10 * 1024 * 1024 // 10MB

// The decode path runs for every OTLP request; pooling the gzip reader and
// the decompressed-body buffer removes a reader setup and the io.ReadAll
// realloc ladder per request. Pooled memory never escapes this file: the
// body bytes are only valid until release() and proto/protojson unmarshal
// copies what it keeps.
var (
	gzipReaderPool sync.Pool
	bodyBufPool    = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

func readBody(c *gin.Context) (body []byte, release func(), err error) {
	var reader io.Reader = c.Request.Body
	if strings.EqualFold(c.GetHeader("Content-Encoding"), "gzip") {
		var gr *gzip.Reader
		if pooled, ok := gzipReaderPool.Get().(*gzip.Reader); ok {
			gr = pooled
			err = gr.Reset(c.Request.Body)
		} else {
			gr, err = gzip.NewReader(c.Request.Body)
		}
		if err != nil {
			if gr != nil {
				gzipReaderPool.Put(gr)
			}
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		// The body is fully consumed below, so the reader can go back to
		// the pool before returning.
		defer func() {
			gr.Close()
			gzipReaderPool.Put(gr)
		}()
		reader = gr
	}

	buf := bodyBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if _, err := buf.ReadFrom(io.LimitReader(reader, maxBodySize)); err != nil {
		bodyBufPool.Put(buf)
		return nil, nil, err
	}
	return buf.Bytes(), func() { bodyBufPool.Put(buf) }, nil
}

func isProtobuf(c *gin.Context) bool {
	ct := c.GetHeader("Content-Type")
	return strings.Contains(ct, "application/x-protobuf") || strings.Contains(ct, "application/protobuf")
}

func decodeTraceRequest(c *gin.Context) (*coltracepb.ExportTraceServiceRequest, int, error) {
	body, release, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
	defer release()
	req := &coltracepb.ExportTraceServiceRequest{}
	if isProtobuf(c) {
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal protobuf: %w", err)
		}
	} else {
		if err := protojson.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return req, len(body), nil
}

func decodeMetricsRequest(c *gin.Context) (*colmetricspb.ExportMetricsServiceRequest, int, error) {
	body, release, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
	defer release()
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if isProtobuf(c) {
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal protobuf: %w", err)
		}
	} else {
		if err := protojson.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return req, len(body), nil
}

func writeTraceResponse(c *gin.Context) {
	resp := &coltracepb.ExportTraceServiceResponse{}
	if isProtobuf(c) {
		data, _ := proto.Marshal(resp)
		c.Data(http.StatusOK, "application/x-protobuf", data)
	} else {
		data, _ := protojson.Marshal(resp)
		c.Data(http.StatusOK, "application/json", data)
	}
}

func writeMetricsResponse(c *gin.Context) {
	resp := &colmetricspb.ExportMetricsServiceResponse{}
	if isProtobuf(c) {
		data, _ := proto.Marshal(resp)
		c.Data(http.StatusOK, "application/x-protobuf", data)
	} else {
		data, _ := protojson.Marshal(resp)
		c.Data(http.StatusOK, "application/json", data)
	}
}

func decodeProfilesPayload(c *gin.Context) ([]byte, int, error) {
	body, release, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
	defer release()
	if isProtobuf(c) {
		// The payload escapes to the profiling pipeline, so it must own its
		// bytes — the pooled buffer is reused after release.
		return bytes.Clone(body), len(body), nil
	}
	req := &colprofilespb.ExportProfilesServiceRequest{}
	if err := protojson.Unmarshal(body, req); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	binary, err := proto.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to re-encode profiles request: %w", err)
	}
	return binary, len(body), nil
}

func writeProfilesResponse(c *gin.Context) {
	resp := &colprofilespb.ExportProfilesServiceResponse{}
	if isProtobuf(c) {
		data, _ := proto.Marshal(resp)
		c.Data(http.StatusOK, "application/x-protobuf", data)
	} else {
		data, _ := protojson.Marshal(resp)
		c.Data(http.StatusOK, "application/json", data)
	}
}

func decodeLogsRequest(c *gin.Context) (*collogspb.ExportLogsServiceRequest, int, error) {
	body, release, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
	defer release()
	req := &collogspb.ExportLogsServiceRequest{}
	if isProtobuf(c) {
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal protobuf: %w", err)
		}
	} else {
		if err := protojson.Unmarshal(body, req); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return req, len(body), nil
}

func writeLogsResponse(c *gin.Context) {
	resp := &collogspb.ExportLogsServiceResponse{}
	if isProtobuf(c) {
		data, _ := proto.Marshal(resp)
		c.Data(http.StatusOK, "application/x-protobuf", data)
	} else {
		data, _ := protojson.Marshal(resp)
		c.Data(http.StatusOK, "application/json", data)
	}
}
