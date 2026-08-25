//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package controllers

import (
	"database/sql"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func createTestOrg(t *testing.T, name string) int {
	t.Helper()
	org, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Organization, error) {
		return transactional.OrganizationRepository.Create(tx, name, "UTC")
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	return org.Id
}

func createTestPostMortem(t *testing.T, organizationId int, projectId uuid.UUID, title string, content string, tags []string, incidentId *int) *models.PostMortem {
	t.Helper()
	now := time.Now().UTC()
	postMortem := &models.PostMortem{
		OrganizationId: organizationId,
		ProjectId:      projectId,
		IncidentId:     incidentId,
		Title:          title,
		ContentMd:      content,
		Tags:           models.StringSlice(tags),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.PostMortemRepository.Create(tx, postMortem)
	})
	if err != nil {
		t.Fatalf("create post-mortem %q: %v", title, err)
	}
	postMortem.Id = id
	return postMortem
}

func TestPostMortemCrudAndSearch(t *testing.T) {
	dbtest.SetupSQLite(t)
	orgId := createTestOrg(t, "org-a")
	otherOrgId := createTestOrg(t, "org-b")
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.CreateWithOrganization(tx, "proj-a", "gin", orgId)
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	otherProject, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.CreateWithOrganization(tx, "proj-b", "gin", otherOrgId)
	})
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	redis := createTestPostMortem(t, orgId, project.Id, "Redis outage", "A cache stampede after failover.", []string{"redis", "cache"}, nil)
	createTestPostMortem(t, orgId, project.Id, "DB failover", "Postgres replication lag caused stale reads.", []string{"postgres"}, nil)

	type listResult struct {
		items []*models.PostMortemListItem
		total int
	}
	list := func(search string, tags ...string) listResult {
		t.Helper()
		result, err := db.ExecuteTransaction(func(tx *sql.Tx) (listResult, error) {
			items, err := transactional.PostMortemRepository.ListByProject(tx, project.Id, search, tags, 50, 0)
			if err != nil {
				return listResult{}, err
			}
			total, err := transactional.PostMortemRepository.CountByProject(tx, project.Id, search, tags)
			return listResult{items: items, total: total}, err
		})
		if err != nil {
			t.Fatalf("list post-mortems (search=%q tags=%v): %v", search, tags, err)
		}
		return result
	}

	if got := list(""); len(got.items) != 2 || got.total != 2 {
		t.Fatalf("unfiltered list: %d items total %d, want 2/2", len(got.items), got.total)
	}
	if got := list("stampede"); len(got.items) != 1 || got.items[0].Title != "Redis outage" {
		t.Fatalf("content search returned %+v", got.items)
	}
	if got := list("REDIS"); len(got.items) != 1 || got.items[0].Title != "Redis outage" {
		t.Fatalf("case-insensitive search returned %+v", got.items)
	}
	if got := list("", "postgres"); len(got.items) != 1 || got.items[0].Title != "DB failover" {
		t.Fatalf("tag filter returned %+v", got.items)
	}
	if got := list("nothing-matches"); len(got.items) != 0 || got.total != 0 {
		t.Fatalf("no-match search returned %+v", got.items)
	}

	if got := list("%"); len(got.items) != 0 || got.total != 0 {
		t.Fatalf("wildcard search matched %+v, want none", got.items)
	}
	if got := list("", "post%"); len(got.items) != 0 || got.total != 0 {
		t.Fatalf("wildcard tag filter matched %+v, want none", got.items)
	}

	foreign, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.PostMortem, error) {
		return transactional.PostMortemRepository.FindByIdForProject(tx, redis.Id, otherProject.Id)
	})
	if err != nil || foreign != nil {
		t.Fatalf("post-mortem leaked across projects: %+v (%v)", foreign, err)
	}

	redis.Title = "Redis outage (updated)"
	redis.Tags = models.StringSlice{"redis"}
	redis.UpdatedAt = time.Now().UTC()
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return true, transactional.PostMortemRepository.Update(tx, redis)
	}); err != nil {
		t.Fatalf("update post-mortem: %v", err)
	}
	reloaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.PostMortem, error) {
		return transactional.PostMortemRepository.FindByIdForProject(tx, redis.Id, project.Id)
	})
	if err != nil || reloaded == nil || reloaded.Title != "Redis outage (updated)" || !slices.Equal(reloaded.Tags, models.StringSlice{"redis"}) {
		t.Fatalf("reloaded post-mortem = %+v (%v)", reloaded, err)
	}

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return true, transactional.PostMortemRepository.Delete(tx, redis.Id, project.Id)
	}); err != nil {
		t.Fatalf("delete post-mortem: %v", err)
	}
	if got := list(""); got.total != 1 {
		t.Fatalf("after delete total = %d, want 1", got.total)
	}
}

func TestIncidentOwnershipAndPostMortemLink(t *testing.T) {
	dbtest.SetupSQLite(t)
	orgId := createTestOrg(t, "org-a")
	otherOrgId := createTestOrg(t, "org-b")

	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.CreateWithOrganization(tx, "proj", "gin", orgId)
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	now := time.Now().UTC()
	check := &models.SyntheticCheck{
		ProjectId:        project.Id,
		Name:             "api",
		CheckType:        models.CheckTypeHttp,
		Enabled:          true,
		IntervalSeconds:  60,
		TimeoutSeconds:   5,
		Config:           models.JSONText(`{}`),
		FailureThreshold: 1,
		CurrentStatus:    models.CheckStatusUnknown,
		NextRunAt:        now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	checkId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.SyntheticCheckRepository.Create(tx, check)
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	check.Id = checkId

	page := &models.StatusPage{
		OrganizationId: orgId,
		Slug:           "acme-status",
		Name:           "Acme Status",
		IsPublic:       true,
		CheckIds:       models.JSONText(`[]`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	pageId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.StatusPageRepository.Create(tx, page)
	})
	if err != nil {
		t.Fatalf("create status page: %v", err)
	}
	page.Id = pageId

	autoIncidentId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.CheckIncidentRepository.Open(tx, &models.CheckIncident{
			CheckId:      &check.Id,
			ProjectId:    &project.Id,
			StartedAt:    now,
			ErrorMessage: "status 500",
		})
	})
	if err != nil {
		t.Fatalf("open auto incident: %v", err)
	}
	manualIncidentId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.CheckIncidentRepository.Open(tx, &models.CheckIncident{
			StatusPageId: &page.Id,
			Title:        "Payment provider outage",
			StartedAt:    now,
		})
	})
	if err != nil {
		t.Fatalf("open manual incident: %v", err)
	}

	for _, incidentId := range []int{autoIncidentId, manualIncidentId} {
		found, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
			return transactional.CheckIncidentRepository.FindByIdInOrganization(tx, incidentId, orgId)
		})
		if err != nil || found == nil {
			t.Fatalf("incident %d not found in owning org: %v", incidentId, err)
		}
		foreign, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
			return transactional.CheckIncidentRepository.FindByIdInOrganization(tx, incidentId, otherOrgId)
		})
		if err != nil || foreign != nil {
			t.Fatalf("incident %d leaked to another org: %+v (%v)", incidentId, foreign, err)
		}
	}

	orgIncidents, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OrgIncident, error) {
		return transactional.CheckIncidentRepository.FindRecentByOrganization(tx, orgId, now.AddDate(0, 0, -90), 100)
	})
	if err != nil || len(orgIncidents) != 2 {
		t.Fatalf("org incidents = %d (%v), want 2", len(orgIncidents), err)
	}

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return true, transactional.CheckIncidentRepository.ResolveManual(tx, autoIncidentId, now)
	}); err != nil {
		t.Fatalf("resolve manual on auto incident: %v", err)
	}
	autoIncident, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
		return transactional.CheckIncidentRepository.FindByIdInOrganization(tx, autoIncidentId, orgId)
	})
	if err != nil || autoIncident == nil || autoIncident.ResolvedAt != nil {
		t.Fatalf("auto incident was hand-resolved: %+v (%v)", autoIncident, err)
	}

	linked := createTestPostMortem(t, orgId, project.Id, "Payment outage write-up", "What happened.", []string{"payments"}, &manualIncidentId)
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.PostMortemRepository.Create(tx, &models.PostMortem{
			OrganizationId: orgId,
			ProjectId:      project.Id,
			IncidentId:     &manualIncidentId,
			Title:          "Duplicate",
			Tags:           models.StringSlice{},
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	})
	if err == nil || !db.IsUniqueViolation(err) {
		t.Fatalf("duplicate incident link error = %v, want unique violation", err)
	}

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return true, transactional.CheckIncidentRepository.DeleteManual(tx, manualIncidentId)
	}); err != nil {
		t.Fatalf("delete manual incident: %v", err)
	}
	survivor, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.PostMortem, error) {
		return transactional.PostMortemRepository.FindByIdForProject(tx, linked.Id, project.Id)
	})
	if err != nil || survivor == nil {
		t.Fatalf("post-mortem did not survive incident deletion: %v", err)
	}
	if survivor.IncidentId != nil {
		t.Fatalf("incident link not severed: %+v", survivor)
	}

	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.IncidentUpdateRepository.Create(tx, &models.IncidentUpdate{
			IncidentId: autoIncidentId,
			Status:     models.IncidentUpdateInvestigating,
			CreatedAt:  now,
		})
	}); err != nil {
		t.Fatalf("create incident update: %v", err)
	}
	if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return true, transactional.SyntheticCheckRepository.Delete(tx, check.Id, project.Id)
	}); err != nil {
		t.Fatalf("delete check: %v", err)
	}
	gone, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.CheckIncident, error) {
		return transactional.CheckIncidentRepository.FindByIdInOrganization(tx, autoIncidentId, orgId)
	})
	if err != nil || gone != nil {
		t.Fatalf("auto incident survived check deletion: %+v (%v)", gone, err)
	}
	updates, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncident(tx, autoIncidentId)
	})
	if err != nil || len(updates) != 0 {
		t.Fatalf("incident updates survived check deletion: %+v (%v)", updates, err)
	}
}

func TestIncidentUpdateBatchHelpers(t *testing.T) {
	dbtest.SetupSQLite(t)
	orgId := createTestOrg(t, "org-a")
	now := time.Now().UTC()
	page := &models.StatusPage{
		OrganizationId: orgId,
		Slug:           "acme-status",
		Name:           "Acme Status",
		IsPublic:       true,
		CheckIds:       models.JSONText(`[]`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	pageId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.StatusPageRepository.Create(tx, page)
	})
	if err != nil {
		t.Fatalf("create status page: %v", err)
	}

	incidentId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.CheckIncidentRepository.Open(tx, &models.CheckIncident{
			StatusPageId: &pageId,
			Title:        "Outage",
			StartedAt:    now,
		})
	})
	if err != nil {
		t.Fatalf("open incident: %v", err)
	}
	for i, status := range []string{models.IncidentUpdateInvestigating, models.IncidentUpdateMonitoring} {
		if _, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
			return transactional.IncidentUpdateRepository.Create(tx, &models.IncidentUpdate{
				IncidentId: incidentId,
				Status:     status,
				CreatedAt:  now.Add(time.Duration(i) * time.Minute),
			})
		}); err != nil {
			t.Fatalf("create update: %v", err)
		}
	}

	empty, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncidentIds(tx, nil)
	})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty id fast path returned %+v (%v)", empty, err)
	}
	batched, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.IncidentUpdate, error) {
		return transactional.IncidentUpdateRepository.FindByIncidentIds(tx, []int{incidentId, incidentId + 100})
	})
	if err != nil || len(batched) != 2 {
		t.Fatalf("batched updates = %d (%v), want 2", len(batched), err)
	}
	counts, err := db.ExecuteTransaction(func(tx *sql.Tx) (map[int]int, error) {
		return transactional.IncidentUpdateRepository.CountByIncidentIds(tx, []int{incidentId})
	})
	if err != nil || counts[incidentId] != 2 {
		t.Fatalf("counts = %v (%v), want map[%d:2]", counts, err, incidentId)
	}
}

func TestNormalizePostMortemTags(t *testing.T) {
	tags, message := normalizePostMortemTags([]string{" Redis ", "redis", `ca"che`, "", `a\b,c`})
	if message != "" {
		t.Fatalf("unexpected validation message: %s", message)
	}
	if !slices.Equal(tags, models.StringSlice{"redis", "cache", "abc"}) {
		t.Fatalf("normalized tags = %v", tags)
	}
	if _, message := normalizePostMortemTags([]string{string(make([]byte, 60))}); message == "" {
		t.Fatal("expected over-long tag to be rejected")
	}
	many := make([]string, 21)
	for i := range many {
		many[i] = "tag" + string(rune('a'+i))
	}
	if _, message := normalizePostMortemTags(many); message == "" {
		t.Fatal("expected too many tags to be rejected")
	}
}
