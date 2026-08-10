package oncall

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const SchemaVersion = 1

const (
	MaxLayersPerSchedule    = 20
	MaxUsersPerLayer        = 50
	MaxRestrictionsPerLayer = 10
	MaxCustomIntervalDays   = 365
	MaxTimelineRangeDays    = 62
	MaxOverrideDurationDays = 30
)

// ParseDefinition parses and validates a schedule definition, assigning ids to
// layers that lack one. Validation errors are user-facing (422) messages.
func ParseDefinition(raw []byte) (*models.OncallScheduleDefinition, error) {
	def := &models.OncallScheduleDefinition{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, def); err != nil {
			return nil, errors.New("The schedule definition is not valid JSON.")
		}
	}
	if def.SchemaVersion != 0 && def.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("Unsupported schedule schemaVersion %d, this server supports schemaVersion %d.", def.SchemaVersion, SchemaVersion)
	}
	def.SchemaVersion = SchemaVersion
	if def.Layers == nil {
		def.Layers = []models.OncallLayer{}
	}
	if len(def.Layers) > MaxLayersPerSchedule {
		return nil, fmt.Errorf("A schedule can have at most %d layers.", MaxLayersPerSchedule)
	}
	for i := range def.Layers {
		if err := validateLayer(&def.Layers[i], i); err != nil {
			return nil, err
		}
	}
	return def, nil
}

func validateLayer(layer *models.OncallLayer, index int) error {
	if layer.Id == "" {
		layer.Id = NewLayerId()
	}
	if layer.Name == "" {
		return fmt.Errorf("Layer %d needs a name.", index+1)
	}
	switch layer.RotationType {
	case RotationDaily:
	case RotationWeekly:
		if layer.HandoffDay < 1 || layer.HandoffDay > 7 {
			return fmt.Errorf("Layer %q needs a handoff day between Monday and Sunday.", layer.Name)
		}
	case RotationCustom:
		if layer.IntervalDays < 1 || layer.IntervalDays > MaxCustomIntervalDays {
			return fmt.Errorf("Layer %q needs a rotation interval between 1 and %d days.", layer.Name, MaxCustomIntervalDays)
		}
	default:
		return fmt.Errorf("Layer %q has an unknown rotation type %q.", layer.Name, layer.RotationType)
	}
	if _, _, ok := parseLocalTime(layer.HandoffTime); !ok {
		return fmt.Errorf("Layer %q needs a handoff time in HH:MM format.", layer.Name)
	}
	if _, _, _, ok := parseLocalDate(layer.RotationStart); !ok {
		return fmt.Errorf("Layer %q needs a rotation start date in YYYY-MM-DD format.", layer.Name)
	}
	if len(layer.UserIds) == 0 {
		return fmt.Errorf("Layer %q needs at least one member.", layer.Name)
	}
	if len(layer.UserIds) > MaxUsersPerLayer {
		return fmt.Errorf("Layer %q can have at most %d members.", layer.Name, MaxUsersPerLayer)
	}
	seen := map[int]bool{}
	for _, userId := range layer.UserIds {
		if seen[userId] {
			return fmt.Errorf("Layer %q lists the same member twice.", layer.Name)
		}
		seen[userId] = true
	}
	if len(layer.Restrictions) > MaxRestrictionsPerLayer {
		return fmt.Errorf("Layer %q can have at most %d restrictions.", layer.Name, MaxRestrictionsPerLayer)
	}
	for _, restriction := range layer.Restrictions {
		if err := validateRestriction(layer.Name, restriction); err != nil {
			return err
		}
	}
	return nil
}

func validateRestriction(layerName string, restriction models.OncallRestriction) error {
	if restriction.Type != RestrictionDaily && restriction.Type != RestrictionWeekly {
		return fmt.Errorf("Layer %q has a restriction with unknown type %q.", layerName, restriction.Type)
	}
	if _, _, ok := parseLocalTime(restriction.StartTime); !ok {
		return fmt.Errorf("Layer %q has a restriction start time that is not HH:MM.", layerName)
	}
	if _, _, ok := parseLocalTime(restriction.EndTime); !ok {
		return fmt.Errorf("Layer %q has a restriction end time that is not HH:MM.", layerName)
	}
	if restriction.Type == RestrictionWeekly {
		if restriction.StartDay < 1 || restriction.StartDay > 7 || restriction.EndDay < 1 || restriction.EndDay > 7 {
			return fmt.Errorf("Layer %q has a weekly restriction with days outside Monday..Sunday.", layerName)
		}
	}
	return nil
}

// LoadTimezone validates an IANA timezone name, returning a user-facing error.
func LoadTimezone(name string) (*time.Location, error) {
	if name == "" {
		return nil, errors.New("A timezone is required.")
	}
	tz, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("Unknown timezone %q.", name)
	}
	return tz, nil
}

func NewLayerId() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "l_" + hex.EncodeToString(b)
}

func MarshalDefinition(def *models.OncallScheduleDefinition) ([]byte, error) {
	return json.Marshal(def)
}
