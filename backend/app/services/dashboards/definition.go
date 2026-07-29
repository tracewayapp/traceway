package dashboards

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const SchemaVersion = 1

const maxWidgetIdLength = 20

const MaxWidgetsPerDashboard = 100

const MaxWidgetConfigBytes = 32 * 1024

var widgetIdPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var ErrUnsupportedSchemaVersion = errors.New("unsupported dashboard schemaVersion")

var AllowedWidgetTypes = []string{"line_chart", "area_chart", "stacked_area", "bar_chart", "single_value", "gauge", "table"}

var allowedWidgetTypeSet = func() map[string]bool {
	set := make(map[string]bool, len(AllowedWidgetTypes))
	for _, t := range AllowedWidgetTypes {
		set[t] = true
	}
	return set
}()

func IsAllowedWidgetType(widgetType string) bool {
	return allowedWidgetTypeSet[widgetType]
}

func SupportedSchemaVersion(v int) bool {
	return v == 0 || v == SchemaVersion
}

func TruncateName(name string, maxRunes int) string {
	if utf8.RuneCountInString(name) <= maxRunes {
		return name
	}
	runes := []rune(name)
	return string(runes[:maxRunes])
}

func NewWidgetId() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "w_" + hex.EncodeToString(b)
}

func ParseDefinition(raw []byte) (*models.DashboardDefinition, error) {
	def := &models.DashboardDefinition{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, def); err != nil {
			return nil, err
		}
	}
	if !SupportedSchemaVersion(def.SchemaVersion) {
		return nil, fmt.Errorf("%w %d, this server supports schemaVersion %d", ErrUnsupportedSchemaVersion, def.SchemaVersion, SchemaVersion)
	}
	if def.SchemaVersion == 0 {
		def.SchemaVersion = SchemaVersion
	}
	if def.Widgets == nil {
		def.Widgets = []models.DashboardWidget{}
	}
	return def, nil
}

func MarshalDefinition(def *models.DashboardDefinition) ([]byte, error) {
	def.SchemaVersion = SchemaVersion
	if def.Widgets == nil {
		def.Widgets = []models.DashboardWidget{}
	}
	return json.Marshal(def)
}

func EnsureWidgetIds(def *models.DashboardDefinition) {
	seen := make(map[string]bool, len(def.Widgets))
	for i := range def.Widgets {
		id := def.Widgets[i].Id
		if id == "" || id == "reorder" || len(id) > maxWidgetIdLength || !widgetIdPattern.MatchString(id) || seen[id] {
			id = NewWidgetId()
			for seen[id] {
				id = NewWidgetId()
			}
			def.Widgets[i].Id = id
		}
		seen[id] = true
	}
}

func FindWidget(def *models.DashboardDefinition, widgetId string) int {
	for i := range def.Widgets {
		if def.Widgets[i].Id == widgetId {
			return i
		}
	}
	return -1
}
