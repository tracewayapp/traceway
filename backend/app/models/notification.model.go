package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type NotificationChannel struct {
	Id          int             `json:"id" lit:"id"`
	ProjectId   uuid.UUID       `json:"projectId" lit:"project_id"`
	Name        string          `json:"name" lit:"name"`
	ChannelType string          `json:"channelType" lit:"channel_type"`
	Config      json.RawMessage `json:"config" lit:"config"`
	Enabled     bool            `json:"enabled" lit:"enabled"`
	CreatedBy   *int            `json:"createdBy" lit:"created_by"`
	CreatedAt   time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt   time.Time       `json:"updatedAt" lit:"updated_at"`
}

type NotificationRule struct {
	Id              int             `json:"id" lit:"id"`
	ProjectId       uuid.UUID       `json:"projectId" lit:"project_id"`
	ChannelId       int             `json:"channelId" lit:"channel_id"`
	Name            string          `json:"name" lit:"name"`
	RuleType        string          `json:"ruleType" lit:"rule_type"`
	Config          json.RawMessage `json:"config" lit:"config"`
	Enabled         bool            `json:"enabled" lit:"enabled"`
	CooldownMinutes int             `json:"cooldownMinutes" lit:"cooldown_minutes"`
	Severity        string          `json:"severity" lit:"severity"`
	SnoozedUntil    *time.Time      `json:"snoozedUntil" lit:"snoozed_until"`
	CreatedBy       *int            `json:"createdBy" lit:"created_by"`
	CreatedAt       time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" lit:"updated_at"`
}

type NotificationHistoryEntry struct {
	RuleId       int       `json:"ruleId"`
	RuleType     string    `json:"ruleType"`
	RuleName     string    `json:"ruleName"`
	ChannelType  string    `json:"channelType"`
	ChannelName  string    `json:"channelName"`
	Severity     string    `json:"severity"`
	Subject      string    `json:"subject"`
	Body         string    `json:"body"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"errorMessage"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"createdAt"`
}

type NotificationRuleWithChannel struct {
	Id              int             `json:"id" lit:"id"`
	ProjectId       uuid.UUID       `json:"projectId" lit:"project_id"`
	ChannelId       int             `json:"channelId" lit:"channel_id"`
	Name            string          `json:"name" lit:"name"`
	RuleType        string          `json:"ruleType" lit:"rule_type"`
	Config          json.RawMessage `json:"config" lit:"config"`
	Enabled         bool            `json:"enabled" lit:"enabled"`
	CooldownMinutes int             `json:"cooldownMinutes" lit:"cooldown_minutes"`
	Severity        string          `json:"severity" lit:"severity"`
	SnoozedUntil    *time.Time      `json:"snoozedUntil" lit:"snoozed_until"`
	CreatedBy       *int            `json:"createdBy" lit:"created_by"`
	CreatedAt       time.Time       `json:"createdAt" lit:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" lit:"updated_at"`
	ChannelName     string          `json:"channelName" lit:"channel_name"`
	ChannelType     string          `json:"channelType" lit:"channel_type"`
}
