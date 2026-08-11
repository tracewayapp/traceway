package oncall

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const (
	MaxPolicySteps       = 10
	MaxTargetsPerStep    = 10
	MinStepDelayMinutes  = 1
	MaxStepDelayMinutes  = 1440
	MaxPolicyRepeatCount = 5
)

const (
	TargetSchedule = "schedule"
	TargetUser     = "user"
	TargetTeam     = "team"
	TargetChannel  = "channel"
)

const UrgencyAuto = "auto"

type PolicyDefinition struct {
	SchemaVersion int          `json:"schemaVersion"`
	Steps         []PolicyStep `json:"steps"`
	RepeatCount   int          `json:"repeatCount"`
	// Urgency picks which of each responder's notification-rule chains runs:
	// ""|"auto" derives from severity, "high"/"low" force it. Optional so
	// stored snapshots and old policies keep parsing (schemaVersion stays 1).
	Urgency string `json:"urgency,omitempty"`
}

// ResolveUrgency maps a policy's urgency setting and a message severity to the
// page urgency. Auto: critical pages are high urgency, everything else low.
// An empty severity counts as critical, mirroring buildPageMessage.
func ResolveUrgency(policyUrgency string, severity string) string {
	switch policyUrgency {
	case models.UrgencyHigh:
		return models.UrgencyHigh
	case models.UrgencyLow:
		return models.UrgencyLow
	}
	if severity == "" || severity == string(models.NotificationSeverityCritical) {
		return models.UrgencyHigh
	}
	return models.UrgencyLow
}

type PolicyStep struct {
	Targets      []StepTarget `json:"targets"`
	DelayMinutes int          `json:"delayMinutes"`
}

type StepTarget struct {
	Type string `json:"type"`
	Id   int    `json:"id"`
}

// ParsePolicyDefinition parses without target-existence checks; used for
// stored snapshots where dangling targets are tolerated at run time.
func ParsePolicyDefinition(raw []byte) (*PolicyDefinition, error) {
	def := &PolicyDefinition{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, def); err != nil {
			return nil, errors.New("The escalation policy definition is not valid JSON.")
		}
	}
	if def.SchemaVersion != 0 && def.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("Unsupported escalation policy schemaVersion %d, this server supports schemaVersion %d.", def.SchemaVersion, SchemaVersion)
	}
	def.SchemaVersion = SchemaVersion
	return def, nil
}

// ValidatePolicyDefinition parses and fully validates a policy definition for
// the organization, including target existence. Errors are user-facing 422
// messages.
func ValidatePolicyDefinition(tx *sql.Tx, organizationId int, raw []byte) (*PolicyDefinition, error) {
	def, err := ParsePolicyDefinition(raw)
	if err != nil {
		return nil, err
	}
	if len(def.Steps) == 0 {
		return nil, errors.New("An escalation policy needs at least one step.")
	}
	if len(def.Steps) > MaxPolicySteps {
		return nil, fmt.Errorf("An escalation policy can have at most %d steps.", MaxPolicySteps)
	}
	if def.RepeatCount < 0 || def.RepeatCount > MaxPolicyRepeatCount {
		return nil, fmt.Errorf("The repeat count must be between 0 and %d.", MaxPolicyRepeatCount)
	}
	switch def.Urgency {
	case "", UrgencyAuto, models.UrgencyHigh, models.UrgencyLow:
	default:
		return nil, errors.New("The urgency must be auto, high, or low.")
	}
	for i, step := range def.Steps {
		if len(step.Targets) == 0 {
			return nil, fmt.Errorf("Step %d needs at least one target.", i+1)
		}
		if len(step.Targets) > MaxTargetsPerStep {
			return nil, fmt.Errorf("Step %d can have at most %d targets.", i+1, MaxTargetsPerStep)
		}
		if step.DelayMinutes < MinStepDelayMinutes || step.DelayMinutes > MaxStepDelayMinutes {
			return nil, fmt.Errorf("Step %d needs a delay between %d and %d minutes.", i+1, MinStepDelayMinutes, MaxStepDelayMinutes)
		}
		for _, target := range step.Targets {
			if err := validateTarget(tx, organizationId, i, target); err != nil {
				return nil, err
			}
		}
	}
	return def, nil
}

func validateTarget(tx *sql.Tx, organizationId int, stepIndex int, target StepTarget) error {
	switch target.Type {
	case TargetSchedule:
		schedule, err := transactional.OncallScheduleRepository.FindById(tx, target.Id)
		if err != nil {
			return err
		}
		if schedule == nil || schedule.OrganizationId != organizationId {
			return fmt.Errorf("Step %d references a schedule that does not exist in this organization.", stepIndex+1)
		}
	case TargetTeam:
		team, err := transactional.TeamRepository.FindById(tx, target.Id)
		if err != nil {
			return err
		}
		if team == nil || team.OrganizationId != organizationId {
			return fmt.Errorf("Step %d references a team that does not exist in this organization.", stepIndex+1)
		}
	case TargetUser:
		role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, target.Id)
		if err != nil {
			return err
		}
		if role == "" {
			return fmt.Errorf("Step %d references a user who is not a member of this organization.", stepIndex+1)
		}
	case TargetChannel:
		channel, err := transactional.NotificationChannelRepository.FindById(tx, target.Id)
		if err != nil {
			return err
		}
		if channel == nil {
			return fmt.Errorf("Step %d references a notification channel that does not exist.", stepIndex+1)
		}
		if channel.ChannelType == "escalation" {
			return fmt.Errorf("Step %d cannot target an escalation channel.", stepIndex+1)
		}
		project, err := transactional.ProjectRepository.FindById(tx, channel.ProjectId)
		if err != nil {
			return err
		}
		if project == nil || project.OrganizationId == nil || *project.OrganizationId != organizationId {
			return fmt.Errorf("Step %d references a notification channel outside this organization.", stepIndex+1)
		}
	default:
		return fmt.Errorf("Step %d has an unknown target type %q.", stepIndex+1, target.Type)
	}
	return nil
}

// PoliciesReferencing returns the names of the organization's escalation
// policies that target any of the given ids of a type; it backs the delete
// guards on schedules and teams.
func PoliciesReferencing(tx *sql.Tx, organizationId int, targetType string, targetIds ...int) ([]string, error) {
	if len(targetIds) == 0 {
		return nil, nil
	}
	wanted := make(map[int]bool, len(targetIds))
	for _, id := range targetIds {
		wanted[id] = true
	}
	policies, err := transactional.EscalationPolicyRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, policy := range policies {
		def, err := ParsePolicyDefinition(policy.Definition)
		if err != nil {
			// Fail closed: an unparseable definition cannot be checked.
			names = append(names, policy.Name)
			continue
		}
		if policyTargets(def, targetType, wanted) {
			names = append(names, policy.Name)
		}
	}
	return names, nil
}

func policyTargets(def *PolicyDefinition, targetType string, wanted map[int]bool) bool {
	for _, step := range def.Steps {
		for _, target := range step.Targets {
			if target.Type == targetType && wanted[target.Id] {
				return true
			}
		}
	}
	return false
}

func MarshalPolicyDefinition(def *PolicyDefinition) ([]byte, error) {
	return json.Marshal(def)
}
