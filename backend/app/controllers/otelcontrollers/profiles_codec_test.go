package otelcontrollers

import (
	"bytes"
	"compress/gzip"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	colprofilespb "go.opentelemetry.io/proto/otlp/collector/profiles/v1development"
	profilespb "go.opentelemetry.io/proto/otlp/profiles/v1development"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func sampleProfilesRequest() *colprofilespb.ExportProfilesServiceRequest {
	return &colprofilespb.ExportProfilesServiceRequest{
		Dictionary: &profilespb.ProfilesDictionary{
			StringTable:   []string{"", "main.main", "cpu", "nanoseconds"},
			FunctionTable: []*profilespb.Function{{}, {NameStrindex: 1}},
			LocationTable: []*profilespb.Location{{}, {Lines: []*profilespb.Line{{FunctionIndex: 1}}}},
			StackTable:    []*profilespb.Stack{{}, {LocationIndices: []int32{1}}},
		},
		ResourceProfiles: []*profilespb.ResourceProfiles{{
			ScopeProfiles: []*profilespb.ScopeProfiles{{
				Profiles: []*profilespb.Profile{{
					SampleType: &profilespb.ValueType{TypeStrindex: 2, UnitStrindex: 3},
					Samples:    []*profilespb.Sample{{StackIndex: 1, Values: []int64{100}}},
				}},
			}},
		}},
	}
}

func newProfilesCtx(t *testing.T, body []byte, contentType, contentEncoding string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/otel/v1development/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func assertDecodedMatches(t *testing.T, payload []byte) {
	t.Helper()
	got := &colprofilespb.ExportProfilesServiceRequest{}
	if err := proto.Unmarshal(payload, got); err != nil {
		t.Fatalf("returned payload is not valid protobuf: %v", err)
	}
	if !proto.Equal(got, sampleProfilesRequest()) {
		t.Errorf("decoded payload does not match original request")
	}
}

func TestDecodeProfilesPayload_Protobuf(t *testing.T) {
	body, err := proto.Marshal(sampleProfilesRequest())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c := newProfilesCtx(t, body, "application/x-protobuf", "")
	payload, n, err := decodeProfilesPayload(c)
	if err != nil {
		t.Fatalf("decodeProfilesPayload: %v", err)
	}
	if n != len(body) {
		t.Errorf("reported body size = %d, want %d", n, len(body))
	}
	assertDecodedMatches(t, payload)
}

func TestDecodeProfilesPayload_ProtoJSON(t *testing.T) {
	body, err := protojson.Marshal(sampleProfilesRequest())
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	c := newProfilesCtx(t, body, "application/json", "")
	payload, _, err := decodeProfilesPayload(c)
	if err != nil {
		t.Fatalf("decodeProfilesPayload: %v", err)
	}
	assertDecodedMatches(t, payload)
}

func TestDecodeProfilesPayload_Gzip(t *testing.T) {
	raw, err := proto.Marshal(sampleProfilesRequest())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	gw.Close()

	c := newProfilesCtx(t, buf.Bytes(), "application/x-protobuf", "gzip")
	payload, _, err := decodeProfilesPayload(c)
	if err != nil {
		t.Fatalf("decodeProfilesPayload: %v", err)
	}
	assertDecodedMatches(t, payload)
}

func TestWriteProfilesResponse_Protobuf(t *testing.T) {
	rec := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/otel/v1development/profiles", nil)
	c.Request.Header.Set("Content-Type", "application/x-protobuf")

	writeProfilesResponse(c)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("content-type = %q, want application/x-protobuf", ct)
	}
	resp := &colprofilespb.ExportProfilesServiceResponse{}
	if err := proto.Unmarshal(rec.Body.Bytes(), resp); err != nil {
		t.Errorf("response not valid protobuf ExportProfilesServiceResponse: %v", err)
	}
}

func TestWriteProfilesResponse_JSON(t *testing.T) {
	rec := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/otel/v1development/profiles", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	writeProfilesResponse(c)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	resp := &colprofilespb.ExportProfilesServiceResponse{}
	if err := protojson.Unmarshal(rec.Body.Bytes(), resp); err != nil {
		t.Errorf("response not valid protojson ExportProfilesServiceResponse: %v", err)
	}
}
