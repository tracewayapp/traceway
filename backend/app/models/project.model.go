package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/config"
	"time"

	"github.com/google/uuid"
)

type StringSlice []string

func (s *StringSlice) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case nil:
		*s = nil
		return nil
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("StringSlice.Scan: unsupported type %T", src)
	}
	if len(data) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(data, s)
}

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

type Project struct {
	Id                      uuid.UUID   `json:"id"`
	Name                    string      `json:"name"`
	Token                   string      `json:"token"`
	Framework               string      `json:"framework"`
	OrganizationId          *int        `json:"organizationId"`
	CreatedAt               time.Time   `json:"createdAt"`
	SourceMapToken          *string     `json:"sourceMapToken,omitempty"`
	DropHealthyHealthchecks bool        `json:"dropHealthyHealthchecks"`
	HealthcheckPaths        StringSlice `json:"healthcheckPaths"`
}

func (p Project) ToProjectWithBackendUrl() *ProjectWithBackendUrl {
	return &ProjectWithBackendUrl{Project: p, BackendUrl: getBackendUrl()}
}

func getBackendUrl() string {
	if url := config.Config.AppBaseURL; url != "" {
		return url
	}
	return "https://cloud.tracewayapp.com"
}

type ProjectWithBackendUrl struct {
	Project
	BackendUrl string `json:"backendUrl"`
}
