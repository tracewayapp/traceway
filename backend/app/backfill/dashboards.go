package backfill

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"

	"github.com/google/uuid"
)

const dashboardsBackfillName = "dashboards"

const maxDashboardNameRunes = 100

type widgetGroupWithOrg struct {
	Id             int       `lit:"id"`
	ProjectId      uuid.UUID `lit:"project_id"`
	Name           string    `lit:"name"`
	Description    string    `lit:"description"`
	IsDefault      bool      `lit:"is_default"`
	CreatedBy      *int      `lit:"created_by"`
	CreatedAt      time.Time `lit:"created_at"`
	UpdatedAt      time.Time `lit:"updated_at"`
	OrganizationId int       `lit:"organization_id"`
	ProjectName    string    `lit:"project_name"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[widgetGroupWithOrg](driver)
	})
}

func RunDashboards() error {
	converted, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824737001)"); err != nil {
				return 0, fmt.Errorf("failed to take backfill lock: %w", err)
			}
		}

		marker, err := lit.SelectSingleNamed[models.CountResult](
			tx,
			"SELECT COUNT(*) as count FROM backfill_runs WHERE name = :name",
			lit.P{"name": dashboardsBackfillName},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to check backfill marker: %w", err)
		}
		if marker != nil && marker.Count > 0 {
			return 0, nil
		}

		existing, err := lit.SelectSingleNamed[models.CountResult](tx, "SELECT COUNT(*) as count FROM dashboards", lit.P{})
		if err != nil {
			return 0, fmt.Errorf("failed to count dashboards: %w", err)
		}

		converted := 0
		if existing == nil || existing.Count == 0 {
			converted, err = convertWidgetGroups(tx)
			if err != nil {
				return 0, err
			}
		}

		query, args, err := lit.ParseNamedQuery(
			db.Driver,
			"INSERT INTO backfill_runs (name, ran_at) VALUES (:name, :ran_at)",
			lit.P{"name": dashboardsBackfillName, "ran_at": time.Now().UTC()},
		)
		if err != nil {
			return 0, err
		}
		if err := lit.UpdateNative(tx, query, args...); err != nil {
			return 0, fmt.Errorf("failed to record backfill marker: %w", err)
		}

		return converted, nil
	})
	if err != nil {
		return err
	}
	if converted > 0 {
		config.Logf("dashboards backfill: converted %d widget group(s) into dashboards", converted)
	}
	return nil
}

func convertWidgetGroups(tx *sql.Tx) (int, error) {
	skipped, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM widget_groups wg JOIN projects p ON p.id = wg.project_id WHERE p.organization_id IS NULL",
		lit.P{},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count orphaned widget groups: %w", err)
	}
	if skipped != nil && skipped.Count > 0 {
		config.Logf("dashboards backfill: skipping %d widget group(s) whose project has no organization; the widget_groups table keeps their data", skipped.Count)
	}

	groups, err := lit.SelectNamed[widgetGroupWithOrg](
		tx,
		`SELECT wg.id, wg.project_id, wg.name, COALESCE(wg.description, '') AS description, wg.is_default, wg.created_by, wg.created_at, wg.updated_at, p.organization_id, p.name AS project_name
		FROM widget_groups wg
		JOIN projects p ON p.id = wg.project_id
		WHERE p.organization_id IS NOT NULL
		ORDER BY wg.project_id ASC, wg.is_default DESC, wg.name ASC`,
		lit.P{},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to list widget groups: %w", err)
	}
	if len(groups) == 0 {
		return 0, nil
	}

	positions := map[uuid.UUID]int{}
	usedNames := map[int]map[string]bool{}
	for _, group := range groups {
		widgets, err := lit.SelectNamed[models.WidgetGroupWidget](
			tx,
			"SELECT id, widget_group_id, title, widget_type, config, position, created_at, updated_at FROM widget_group_widgets WHERE widget_group_id = :wg_id ORDER BY position ASC",
			lit.P{"wg_id": group.Id},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to list widgets for group %d: %w", group.Id, err)
		}

		def := &models.DashboardDefinition{SchemaVersion: dashboardsvc.SchemaVersion}
		widgetIds := make(map[int]string, len(widgets))
		usedIds := make(map[string]bool, len(widgets))
		for _, w := range widgets {
			uid := dashboardsvc.NewWidgetId()
			for usedIds[uid] {
				uid = dashboardsvc.NewWidgetId()
			}
			usedIds[uid] = true
			widgetIds[w.Id] = uid
			widgetConfig := json.RawMessage(w.Config)
			if len(widgetConfig) == 0 {
				widgetConfig = json.RawMessage(`{}`)
			}
			def.Widgets = append(def.Widgets, models.DashboardWidget{
				Id:         uid,
				Title:      w.Title,
				WidgetType: w.WidgetType,
				Config:     widgetConfig,
			})
		}
		definition, err := dashboardsvc.MarshalDefinition(def)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal definition for group %d: %w", group.Id, err)
		}

		orgNames := usedNames[group.OrganizationId]
		if orgNames == nil {
			orgNames = map[string]bool{}
			usedNames[group.OrganizationId] = orgNames
		}
		name := uniqueDashboardName(orgNames, group.Name, group.ProjectName)
		orgNames[strings.ToLower(name)] = true

		dashboard := &models.Dashboard{
			OrganizationId: group.OrganizationId,
			Name:           name,
			Description:    group.Description,
			Definition:     definition,
			CreatedBy:      group.CreatedBy,
			CreatedAt:      group.CreatedAt,
			UpdatedAt:      group.UpdatedAt,
		}
		dashboardId, err := transactional.DashboardRepository.Create(tx, dashboard)
		if err != nil {
			return 0, fmt.Errorf("failed to create dashboard for group %d: %w", group.Id, err)
		}

		position := positions[group.ProjectId]
		positions[group.ProjectId] = position + 1
		if err := transactional.DashboardRepository.CreateAssignment(tx, &models.ProjectDashboard{
			ProjectId:   group.ProjectId,
			DashboardId: dashboardId,
			Position:    position,
			CreatedAt:   group.CreatedAt,
		}); err != nil {
			return 0, fmt.Errorf("failed to assign dashboard for group %d: %w", group.Id, err)
		}

		starred, err := lit.SelectNamed[models.StarredWidget](
			tx,
			`SELECT sw.id, sw.widget_id, sw.position, sw.col_span, sw.size, sw.created_at
			FROM starred_widgets sw
			JOIN widget_group_widgets wgw ON wgw.id = sw.widget_id
			WHERE wgw.widget_group_id = :wg_id
			ORDER BY sw.position ASC, sw.id ASC`,
			lit.P{"wg_id": group.Id},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to list starred widgets for group %d: %w", group.Id, err)
		}
		for _, s := range starred {
			uid := widgetIds[s.WidgetId]
			if uid == "" {
				continue
			}
			if err := transactional.DashboardRepository.CreateStarred(tx, &models.StarredDashboardWidget{
				ProjectId:   group.ProjectId,
				DashboardId: dashboardId,
				WidgetId:    uid,
				Position:    s.Position,
				ColSpan:     s.ColSpan,
				Size:        s.Size,
				CreatedAt:   s.CreatedAt,
			}); err != nil {
				return 0, fmt.Errorf("failed to migrate starred widget %d for group %d: %w", s.Id, group.Id, err)
			}
		}
	}

	return len(groups), nil
}

func uniqueDashboardName(used map[string]bool, groupName string, projectName string) string {
	base := dashboardsvc.TruncateName(groupName, maxDashboardNameRunes)
	if !used[strings.ToLower(base)] {
		return base
	}
	candidate := suffixDashboardName(base, projectName, 1)
	if !used[strings.ToLower(candidate)] {
		return candidate
	}
	for n := 2; ; n++ {
		candidate = suffixDashboardName(base, projectName, n)
		if !used[strings.ToLower(candidate)] {
			return candidate
		}
	}
}

func suffixDashboardName(base string, projectName string, n int) string {
	numeral := ""
	if n > 1 {
		numeral = fmt.Sprintf(" %d", n)
	}
	maxProjectRunes := maxDashboardNameRunes - 4 - utf8.RuneCountInString(numeral)
	suffix := " (" + dashboardsvc.TruncateName(projectName, maxProjectRunes) + ")" + numeral
	limit := maxDashboardNameRunes - utf8.RuneCountInString(suffix)
	if limit < 1 {
		limit = 1
	}
	return dashboardsvc.TruncateName(base, limit) + suffix
}
