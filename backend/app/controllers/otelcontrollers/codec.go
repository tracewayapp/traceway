package otelcontrollers

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
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

func decodeTraceRequest(c *gin.Context) (*coltracepb.ExportTraceServiceRequest, int, error) {
	body, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
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
	body, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
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
	body, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
	if isProtobuf(c) {
		return body, len(body), nil
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
	body, err := readBody(c)
	if err != nil {
		return nil, 0, err
	}
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
