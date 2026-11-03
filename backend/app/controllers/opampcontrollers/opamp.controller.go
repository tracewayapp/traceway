// Package opampcontrollers serves OpAMP (Open Agent Management Protocol) over plain HTTP so OpenTelemetry SDKs,
// the browser SDK first, receive their collection settings from Traceway. It implements only the HTTP polling
// transport and is stateless: every poll is answered from the agent description it carries, so any replica can
// answer any client and nothing is stored per client.
package opampcontrollers

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/sdkconfig"
	traceway "go.tracewayapp.com"
	"google.golang.org/protobuf/proto"
)

const (
	contentType = "application/x-protobuf"
	// ConfigMapKey names the settings document in AgentRemoteConfig.config_map.
	ConfigMapKey       = "traceway"
	maxBodyBytes       = 64 << 10
	maxAttributes      = 128
	maxAttributeKey    = 128
	maxAttributeValue  = 1024
	captureHeartbeat   = 60
	unavailableRetryIn = 60 * time.Second
	instanceUidHeader  = "OpAMP-Instance-UID"
)

var serverCapabilities = uint64(protobufs.ServerCapabilities_ServerCapabilities_AcceptsStatus |
	protobufs.ServerCapabilities_ServerCapabilities_OffersRemoteConfig |
	protobufs.ServerCapabilities_ServerCapabilities_OffersConnectionSettings)

var errBodyTooLarge = errors.New("OpAMP message too large")

type opampController struct{}

var OpampController = opampController{}

func (opampController) Poll(c *gin.Context) {
	if c.ContentType() != contentType {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "OpAMP over HTTP needs Content-Type: application/x-protobuf; WebSocket is not supported"})
		return
	}
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	project, _ := c.Get(middleware.ProjectContextKey)
	browserProject, ok := project.(*models.Project)
	if !ok || browserProject == nil || !sdkconfig.SupportsFramework(browserProject.Framework) {
		c.JSON(http.StatusForbidden, gin.H{"error": "OpAMP is only available for browser frontend projects"})
		return
	}

	middleware.GuardBodyRead(c, 5*time.Second, 15*time.Second)
	body, err := readBody(c)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			// OpAMP clients must not retry a 413.
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		if status, message, ok := middleware.BodyReadError(err); ok {
			c.JSON(status, gin.H{"error": message})
			return
		}
		writeError(c, http.StatusBadRequest, protobufs.ServerErrorResponseType_ServerErrorResponseType_BadRequest, "unreadable body")
		return
	}

	var msg protobufs.AgentToServer
	if err := proto.Unmarshal(body, &msg); err != nil {
		writeError(c, http.StatusBadRequest, protobufs.ServerErrorResponseType_ServerErrorResponseType_BadRequest, "body is not an AgentToServer message")
		return
	}
	uid, ok := instanceUid(c, msg.InstanceUid)
	if !ok {
		writeError(c, http.StatusBadRequest, protobufs.ServerErrorResponseType_ServerErrorResponseType_BadRequest, "instance_uid must be 16 bytes and match the OpAMP-Instance-UID header")
		return
	}

	resp := &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid, Capabilities: serverCapabilities}
	if msg.Flags&uint64(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid) != 0 {
		assigned := uuid.Must(uuid.NewV7())
		resp.AgentIdentification = &protobufs.AgentIdentification{NewInstanceUid: assigned[:]}
		uid = assigned
	}
	if msg.AgentDisconnect != nil {
		write(c, resp)
		return
	}
	// The server keeps no state per client, so a client that left its description out of this poll (OpAMP lets
	// it omit unchanged fields) is asked to send everything again.
	if msg.AgentDescription == nil {
		resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
		write(c, resp)
		return
	}

	entry, err := cache.SdkConfigCache.Get(projectId)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("loading SDK config for OpAMP poll: %w", err))
		c.Header("Retry-After", strconv.Itoa(int(unavailableRetryIn.Seconds())))
		resp.ErrorResponse = &protobufs.ServerErrorResponse{
			Type:         protobufs.ServerErrorResponseType_ServerErrorResponseType_Unavailable,
			ErrorMessage: "remote config is temporarily unavailable",
			Details: &protobufs.ServerErrorResponse_RetryInfo{
				RetryInfo: &protobufs.RetryInfo{RetryAfterNanoseconds: uint64(unavailableRetryIn.Nanoseconds())},
			},
		}
		writeStatus(c, http.StatusServiceUnavailable, resp)
		return
	}

	result := entry.Compiled.Evaluate(DescriptionAttributes(msg.AgentDescription), uid, time.Now())
	if perm, ok := ingestPermission(c); ok {
		sdkconfig.ApplyPermission(&result.Settings, perm.Exceptions, perm.Data, perm.Replay)
	}
	document, hash, err := sdkconfig.Render(result.Settings)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("rendering SDK config: %w", err))
		return
	}

	heartbeat := config.OpampHeartbeatSeconds()
	// A client under an active capture polls more often, so cancelling or extending the capture reaches it soon.
	if len(result.MatchedCaptures) > 0 {
		heartbeat = min(heartbeat, captureHeartbeat)
	}
	heartbeatHash := sha256.Sum256([]byte(strconv.FormatUint(heartbeat, 10)))
	resp.ConnectionSettings = &protobufs.ConnectionSettingsOffers{
		Hash:  heartbeatHash[:],
		Opamp: &protobufs.OpAMPConnectionSettings{HeartbeatIntervalSeconds: heartbeat},
	}

	acceptsRemoteConfig := msg.Capabilities&uint64(protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig) != 0
	if acceptsRemoteConfig && !bytes.Equal(msg.RemoteConfigStatus.GetLastRemoteConfigHash(), hash) {
		resp.RemoteConfig = &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigObject{
				ConfigMapKey: {Body: document, ContentType: "application/json"},
			}},
			ConfigHash: hash,
		}
	}
	write(c, resp)
}

func readBody(c *gin.Context) ([]byte, error) {
	reader := io.Reader(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
	if strings.EqualFold(c.GetHeader("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return nil, errBodyTooLarge
		}
		return nil, err
	}
	if len(body) > maxBodyBytes {
		return nil, errBodyTooLarge
	}
	return body, nil
}

func instanceUid(c *gin.Context, raw []byte) (uuid.UUID, bool) {
	if len(raw) != 16 {
		return uuid.Nil, false
	}
	uid := uuid.UUID(raw)
	if header := c.GetHeader(instanceUidHeader); header != "" {
		parsed, err := uuid.Parse(header)
		if err != nil || parsed != uid {
			return uuid.Nil, false
		}
	}
	return uid, true
}

// DescriptionAttributes flattens an agent description into the attributes rules match on. Identifying attributes
// win over non-identifying ones of the same key; composite values are matched as their JSON text.
func DescriptionAttributes(description *protobufs.AgentDescription) map[string]string {
	attrs := map[string]string{}
	add := func(kvs []*protobufs.KeyValue) {
		for _, kv := range kvs {
			if len(attrs) >= maxAttributes || kv.GetKey() == "" || len(kv.GetKey()) > maxAttributeKey {
				continue
			}
			value, ok := anyValueString(kv.GetValue())
			if !ok || len(value) > maxAttributeValue {
				continue
			}
			attrs[kv.GetKey()] = value
		}
	}
	add(description.GetNonIdentifyingAttributes())
	add(description.GetIdentifyingAttributes())
	return attrs
}

func anyValueString(v *protobufs.AnyValue) (string, bool) {
	switch value := v.GetValue().(type) {
	case *protobufs.AnyValue_StringValue:
		return value.StringValue, true
	case *protobufs.AnyValue_BoolValue:
		return strconv.FormatBool(value.BoolValue), true
	case *protobufs.AnyValue_IntValue:
		return strconv.FormatInt(value.IntValue, 10), true
	case *protobufs.AnyValue_DoubleValue:
		return strconv.FormatFloat(value.DoubleValue, 'g', -1, 64), true
	case *protobufs.AnyValue_ArrayValue:
		items := []string{}
		for _, item := range value.ArrayValue.GetValues() {
			if s, ok := anyValueString(item); ok {
				items = append(items, s)
			}
		}
		encoded, err := json.Marshal(items)
		return string(encoded), err == nil
	}
	return "", false
}

func ingestPermission(c *gin.Context) (hooks.IngestPermission, bool) {
	value, exists := c.Get(middleware.ProjectContextKey)
	if !exists {
		return hooks.IngestPermission{}, false
	}
	project, ok := value.(*models.Project)
	if !ok || project.OrganizationId == nil {
		return hooks.IngestPermission{}, false
	}
	return hooks.IngestPermissionFor(*project.OrganizationId), true
}

func writeError(c *gin.Context, status int, kind protobufs.ServerErrorResponseType, message string) {
	writeStatus(c, status, &protobufs.ServerToAgent{
		ErrorResponse: &protobufs.ServerErrorResponse{Type: kind, ErrorMessage: message},
	})
}

func write(c *gin.Context, resp *protobufs.ServerToAgent) {
	writeStatus(c, http.StatusOK, resp)
}

func writeStatus(c *gin.Context, status int, resp *protobufs.ServerToAgent) {
	out, err := proto.Marshal(resp)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("encoding ServerToAgent: %w", err))
		return
	}
	c.Data(status, contentType, out)
}
