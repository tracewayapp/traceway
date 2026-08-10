package models

import (
	"time"

	"github.com/google/uuid"
)

type Team struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	Name           string    `json:"name" lit:"name"`
	Description    string    `json:"description" lit:"description"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
}

type TeamMember struct {
	Id        int       `json:"id" lit:"id"`
	TeamId    int       `json:"teamId" lit:"team_id"`
	UserId    int       `json:"userId" lit:"user_id"`
	Position  int       `json:"position" lit:"position"`
	CreatedAt time.Time `json:"createdAt" lit:"created_at"`
}

type TeamMemberWithUser struct {
	TeamId   int    `json:"teamId" lit:"team_id"`
	UserId   int    `json:"userId" lit:"user_id"`
	Position int    `json:"position" lit:"position"`
	Name     string `json:"name" lit:"name"`
	Email    string `json:"email" lit:"email"`
}

type ProjectTeam struct {
	Id        int       `json:"id" lit:"id"`
	ProjectId uuid.UUID `json:"projectId" lit:"project_id"`
	TeamId    int       `json:"teamId" lit:"team_id"`
	CreatedAt time.Time `json:"createdAt" lit:"created_at"`
}

type OncallSchedule struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	TeamId         int       `json:"teamId" lit:"team_id"`
	Name           string    `json:"name" lit:"name"`
	Description    string    `json:"description" lit:"description"`
	Timezone       string    `json:"timezone" lit:"timezone"`
	Definition     JSONText  `json:"definition" lit:"definition"`
	CreatedBy      *int      `json:"createdBy" lit:"created_by"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
}

type OncallOverride struct {
	Id         int       `json:"id" lit:"id"`
	ScheduleId int       `json:"scheduleId" lit:"schedule_id"`
	UserId     int       `json:"userId" lit:"user_id"`
	StartAt    time.Time `json:"startAt" lit:"start_at"`
	EndAt      time.Time `json:"endAt" lit:"end_at"`
	CreatedBy  *int      `json:"createdBy" lit:"created_by"`
	CreatedAt  time.Time `json:"createdAt" lit:"created_at"`
}

type TeamWithCounts struct {
	Id             int       `json:"id" lit:"id"`
	OrganizationId int       `json:"organizationId" lit:"organization_id"`
	Name           string    `json:"name" lit:"name"`
	Description    string    `json:"description" lit:"description"`
	CreatedAt      time.Time `json:"createdAt" lit:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" lit:"updated_at"`
	MemberCount    int       `json:"memberCount" lit:"member_count"`
	ProjectCount   int       `json:"projectCount" lit:"project_count"`
	ScheduleCount  int       `json:"scheduleCount" lit:"schedule_count"`
}

type TeamProjectRow struct {
	TeamId    int       `json:"teamId" lit:"team_id"`
	ProjectId uuid.UUID `json:"projectId" lit:"project_id"`
	Name      string    `json:"name" lit:"name"`
}

type OncallScheduleDefinition struct {
	SchemaVersion int           `json:"schemaVersion"`
	Layers        []OncallLayer `json:"layers"`
}

type OncallLayer struct {
	Id            string              `json:"id"`
	Name          string              `json:"name"`
	RotationType  string              `json:"rotationType"`
	HandoffTime   string              `json:"handoffTime"`
	HandoffDay    int                 `json:"handoffDay"`
	IntervalDays  int                 `json:"intervalDays"`
	RotationStart string              `json:"rotationStart"`
	UserIds       []int               `json:"userIds"`
	Restrictions  []OncallRestriction `json:"restrictions"`
}

type OncallRestriction struct {
	Type      string `json:"type"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	StartDay  int    `json:"startDay"`
	EndDay    int    `json:"endDay"`
}
