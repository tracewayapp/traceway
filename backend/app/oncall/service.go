package oncall

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

type ScheduleRef struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type OncallUser struct {
	UserId int    `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// ProjectOnCall is the ownership seam: the team owning a project and who is on
// call for it right now.
type ProjectOnCall struct {
	Team      *models.Team  `json:"team"`
	Schedules []ScheduleRef `json:"schedules"`
	Oncall    []OncallUser  `json:"oncall"`
}

// CurrentOnCallForSchedule resolves who is on call for a schedule at the given
// instant, filtered to current members of the schedule's organization. A
// missing schedule resolves to nobody rather than an error, so dangling
// references never abort a caller.
func CurrentOnCallForSchedule(tx *sql.Tx, scheduleId int, at time.Time) ([]int, error) {
	schedule, err := transactional.OncallScheduleRepository.FindById(tx, scheduleId)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, nil
	}
	userIds, err := resolveScheduleAt(tx, schedule, at)
	if err != nil {
		return nil, err
	}
	members, err := memberDetails(tx, schedule.OrganizationId)
	if err != nil {
		return nil, err
	}
	var filtered []int
	for _, userId := range userIds {
		if _, ok := members[userId]; ok {
			filtered = append(filtered, userId)
		}
	}
	return filtered, nil
}

// RemoveUserFromOrgSchedules scrubs a removed member from the organization's
// on-call data: their overrides are deleted and they are dropped from every
// schedule layer (a layer left with no members is dropped with them, keeping
// stored definitions valid). Resolution already filters to current org
// members; this keeps schedules and timelines from showing phantom coverage.
func RemoveUserFromOrgSchedules(tx *sql.Tx, organizationId int, userId int) error {
	if err := transactional.OncallOverrideRepository.DeleteByOrganizationAndUser(tx, organizationId, userId); err != nil {
		return err
	}
	schedules, err := transactional.OncallScheduleRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, schedule := range schedules {
		def, err := ParseDefinition(schedule.Definition)
		if err != nil {
			// An unparseable definition cannot be scrubbed; resolution filters
			// removed members out regardless.
			traceway.CaptureException(fmt.Errorf("cannot scrub user %d from schedule %d: %w", userId, schedule.Id, err))
			continue
		}
		changed := false
		layers := make([]models.OncallLayer, 0, len(def.Layers))
		for _, layer := range def.Layers {
			userIds := make([]int, 0, len(layer.UserIds))
			for _, id := range layer.UserIds {
				if id == userId {
					changed = true
					continue
				}
				userIds = append(userIds, id)
			}
			if len(userIds) == 0 {
				continue
			}
			layer.UserIds = userIds
			layers = append(layers, layer)
		}
		if !changed {
			continue
		}
		def.Layers = layers
		raw, err := MarshalDefinition(def)
		if err != nil {
			return err
		}
		schedule.Definition = raw
		schedule.UpdatedAt = now
		if err := transactional.OncallScheduleRepository.Update(tx, schedule); err != nil {
			return err
		}
	}
	return nil
}

// CurrentOnCallForProject resolves the owning team's current on-call across
// all of its schedules (schedule creation order), filtered to current org
// members. Returns nil when the project has no owning team.
func CurrentOnCallForProject(tx *sql.Tx, projectId uuid.UUID, at time.Time) (*ProjectOnCall, error) {
	team, err := transactional.TeamRepository.FindTeamForProject(tx, projectId)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	schedules, err := transactional.OncallScheduleRepository.ListByTeam(tx, team.Id)
	if err != nil {
		return nil, err
	}
	members, err := memberDetails(tx, team.OrganizationId)
	if err != nil {
		return nil, err
	}

	result := &ProjectOnCall{Team: team, Schedules: []ScheduleRef{}, Oncall: []OncallUser{}}
	seen := map[int]bool{}
	for _, schedule := range schedules {
		result.Schedules = append(result.Schedules, ScheduleRef{Id: schedule.Id, Name: schedule.Name})
		userIds, err := resolveScheduleAt(tx, schedule, at)
		if err != nil {
			return nil, err
		}
		for _, userId := range userIds {
			member, ok := members[userId]
			if !ok || seen[userId] {
				continue
			}
			seen[userId] = true
			result.Oncall = append(result.Oncall, OncallUser{UserId: userId, Name: member.Name, Email: member.Email})
		}
	}
	return result, nil
}

func resolveScheduleAt(tx *sql.Tx, schedule *models.OncallSchedule, at time.Time) ([]int, error) {
	tz, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		tz = time.UTC
	}
	def, err := ParseDefinition(schedule.Definition)
	if err != nil {
		// A stored definition that no longer parses must not wedge a page's
		// escalation (the claim would fail and retry forever): treat it like a
		// dangling target — report it and resolve to nobody, so other targets
		// and later levels still run.
		traceway.CaptureException(fmt.Errorf("schedule %d definition no longer parses, resolving to nobody on call: %w", schedule.Id, err))
		return nil, nil
	}
	overrides, err := transactional.OncallOverrideRepository.ListForRange(tx, schedule.Id, at, at.Add(time.Nanosecond))
	if err != nil {
		return nil, err
	}
	return ResolveAt(def, tz, overrides, at), nil
}

func memberDetails(tx *sql.Tx, organizationId int) (map[int]*models.OrganizationMember, error) {
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		return nil, err
	}
	byId := make(map[int]*models.OrganizationMember, len(members))
	for _, member := range members {
		byId[member.Id] = member
	}
	return byId, nil
}
