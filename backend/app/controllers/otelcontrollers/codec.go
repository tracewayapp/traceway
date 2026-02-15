package otelcontrollers

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxBodySize = 10 * 1024 * 1024 // 10MB

func readBody(c *gin.Context) ([]byte, error) {
	var reader io.Reader = c.Request.Body
	if strings.EqualFold(c.GetHeader("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	}
	return io.ReadAll(io.LimitReader(reader, maxBodySize))
}

func isProtobuf(c *gin.Context) bool {
	ct := c.GetHeader("Content-Type")
	return strings.Contains(ct, "application/x-protobuf") || strings.Contains(ct, "application/protobuf")
}

func decodeTraceRequest(c *gin.Context) (*coltracepb.ExportTraceServiceRequest, error) {
	body, err := readBody(c)
	if err != nil {
		return nil, err
	}
	req := &coltracepb.ExportTraceServiceRequest{}
	if isProtobuf(c) {
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
		}
	} else {
		if err := protojson.Unmarshal(body, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return req, nil
}

func decodeMetricsRequest(c *gin.Context) (*colmetricspb.ExportMetricsServiceRequest, error) {
	body, err := readBody(c)
	if err != nil {
		return nil, err
	}
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if isProtobuf(c) {
		if err := proto.Unmarshal(body, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
		}
	} else {
		if err := protojson.Unmarshal(body, req); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return req, nil
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
