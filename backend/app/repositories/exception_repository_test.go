package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestExceptionRepository_InsertAndCount(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "hash-a", "RuntimeError: something failed\n  at main.go:42", now),
		makeException(projectId, "hash-a", "RuntimeError: something failed\n  at main.go:42", now.Add(time.Minute)),
		makeException(projectId, "hash-b", "NullPointerError: nil reference\n  at handler.go:10", now.Add(2*time.Minute)),
	}

	err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions)
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := ExceptionStackTraceRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestExceptionRepository_CountBetween_TimeFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "old-hash", "old error", now.Add(-2*time.Hour)),
		makeException(projectId, "new-hash", "new error", now),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := ExceptionStackTraceRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1 (only recent exception), got %d", count)
	}
}

func TestExceptionRepository_FindGrouped(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "hash-a", "Error A\n  at main.go:1", now),
		makeException(projectId, "hash-a", "Error A\n  at main.go:1", now.Add(time.Minute)),
		makeException(projectId, "hash-a", "Error A\n  at main.go:1", now.Add(2*time.Minute)),
		makeException(projectId, "hash-b", "Error B\n  at handler.go:5", now.Add(3*time.Minute)),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	groups, total, err := ExceptionStackTraceRepository.FindGrouped(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "", "", false)
	if err != nil {
		t.Fatalf("FindGrouped failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 groups, got %d", total)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 group entries, got %d", len(groups))
	}

	// Ordered by count DESC: hash-a (3) first
	if groups[0].ExceptionHash != "hash-a" {
		t.Errorf("expected first group hash 'hash-a', got %q", groups[0].ExceptionHash)
	}
	if groups[0].Count != 3 {
		t.Errorf("expected count 3, got %d", groups[0].Count)
	}
}

func TestExceptionRepository_FindGrouped_Search(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "hash-db", "DatabaseError: connection refused\n  at db.go:42", now),
		makeException(projectId, "hash-nil", "NilPointerError: nil reference\n  at handler.go:10", now.Add(time.Minute)),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	groups, total, err := ExceptionStackTraceRepository.FindGrouped(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "Database", "", false)
	if err != nil {
		t.Fatalf("FindGrouped with search failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 group matching search, got %d", total)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group entry, got %d", len(groups))
	}
	if groups[0].ExceptionHash != "hash-db" {
		t.Errorf("expected hash 'hash-db', got %q", groups[0].ExceptionHash)
	}
}

func TestExceptionRepository_FindByHash(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "target-hash", "RuntimeError: fail\n  at main.go:1", now),
		makeException(projectId, "target-hash", "RuntimeError: fail\n  at main.go:1", now.Add(time.Minute)),
		makeException(projectId, "other-hash", "OtherError: other\n  at other.go:1", now.Add(2*time.Minute)),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	group, occurrences, total, err := ExceptionStackTraceRepository.FindByHash(ctx, projectId, "target-hash", 1, 10)
	if err != nil {
		t.Fatalf("FindByHash failed: %v", err)
	}

	if group == nil {
		t.Fatal("expected group, got nil")
	}
	if group.Count != 2 {
		t.Errorf("expected group count 2, got %d", group.Count)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(occurrences) != 2 {
		t.Errorf("expected 2 occurrences, got %d", len(occurrences))
	}
}

func TestExceptionRepository_ArchiveAndUnarchive(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "archive-hash", "Error to archive", now),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	// Initially not archived
	archived, err := ExceptionStackTraceRepository.IsArchived(ctx, projectId, "archive-hash")
	if err != nil {
		t.Fatalf("IsArchived failed: %v", err)
	}
	if archived {
		t.Error("expected not archived initially")
	}

	// Archive
	if err := ExceptionStackTraceRepository.ArchiveByHashes(ctx, projectId, []string{"archive-hash"}); err != nil {
		t.Fatalf("ArchiveByHashes failed: %v", err)
	}

	archived, err = ExceptionStackTraceRepository.IsArchived(ctx, projectId, "archive-hash")
	if err != nil {
		t.Fatalf("IsArchived after archive failed: %v", err)
	}
	if !archived {
		t.Error("expected archived after ArchiveByHashes")
	}

	// Unarchive
	if err := ExceptionStackTraceRepository.UnarchiveByHashes(ctx, projectId, []string{"archive-hash"}); err != nil {
		t.Fatalf("UnarchiveByHashes failed: %v", err)
	}

	archived, err = ExceptionStackTraceRepository.IsArchived(ctx, projectId, "archive-hash")
	if err != nil {
		t.Fatalf("IsArchived after unarchive failed: %v", err)
	}
	if archived {
		t.Error("expected not archived after UnarchiveByHashes")
	}
}

func TestExceptionRepository_FindGrouped_ExcludesArchived(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "active-hash", "Active error", now),
		makeException(projectId, "archived-hash", "Archived error", now.Add(-time.Minute)),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	if err := ExceptionStackTraceRepository.ArchiveByHashes(ctx, projectId, []string{"archived-hash"}); err != nil {
		t.Fatalf("ArchiveByHashes failed: %v", err)
	}

	// With includeArchived=false, should only see active
	groups, total, err := ExceptionStackTraceRepository.FindGrouped(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "", "", false)
	if err != nil {
		t.Fatalf("FindGrouped failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 group (excluding archived), got %d", total)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group entry, got %d", len(groups))
	}
	if groups[0].ExceptionHash != "active-hash" {
		t.Errorf("expected 'active-hash', got %q", groups[0].ExceptionHash)
	}
}

func TestExceptionRepository_FindById(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	exc := makeException(projectId, "find-id-hash", "Error for FindById", now)

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{exc}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := ExceptionStackTraceRepository.FindById(ctx, projectId, exc.Id, nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find exception, got nil")
	}
	if found.ExceptionHash != "find-id-hash" {
		t.Errorf("expected hash 'find-id-hash', got %q", found.ExceptionHash)
	}
}

func TestExceptionRepository_FindById_NotFound(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	found, err := ExceptionStackTraceRepository.FindById(ctx, uuid.New(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for unknown exception, got %+v", found)
	}
}

func TestExceptionRepository_GetHourlyTrendForHashes(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	exceptions := []models.ExceptionStackTrace{
		makeException(projectId, "trend-hash", "Error trend", base),
		makeException(projectId, "trend-hash", "Error trend", base.Add(30*time.Minute)),
		makeException(projectId, "trend-hash", "Error trend", base.Add(time.Hour+10*time.Minute)),
	}

	if err := ExceptionStackTraceRepository.InsertAsync(ctx, exceptions); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	trends, err := ExceptionStackTraceRepository.GetHourlyTrendForHashes(ctx, projectId, []string{"trend-hash"}, base.Add(-time.Minute), base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("GetHourlyTrendForHashes failed: %v", err)
	}

	points, ok := trends["trend-hash"]
	if !ok {
		t.Fatal("expected trend data for 'trend-hash'")
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 hourly trend points, got %d", len(points))
	}

	if points[0].Count != 2 {
		t.Errorf("expected 2 exceptions in first hour, got %d", points[0].Count)
	}
	if points[1].Count != 1 {
		t.Errorf("expected 1 exception in second hour, got %d", points[1].Count)
	}
}

func TestExceptionRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}
