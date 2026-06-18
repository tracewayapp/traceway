package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestTaskRepository_InsertAndCount(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "email.send", 100*time.Millisecond, now),
		makeTask(projectId, "email.send", 200*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "report.generate", 500*time.Millisecond, now.Add(2*time.Minute)),
	}

	err := TaskRepository.InsertAsync(ctx, tasks)
	if err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := TaskRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestTaskRepository_CountBetween_TimeFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "task-old", 100*time.Millisecond, now.Add(-2*time.Hour)),
		makeTask(projectId, "task-recent", 100*time.Millisecond, now),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	count, err := TaskRepository.CountBetween(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CountBetween failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1 (only recent task), got %d", count)
	}
}

func TestTaskRepository_FindAll(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "task-a", 100*time.Millisecond, now),
		makeTask(projectId, "task-b", 200*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "task-c", 300*time.Millisecond, now.Add(2*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, total, err := TaskRepository.FindAll(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "recorded_at")
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(found) != 3 {
		t.Errorf("expected 3 results, got %d", len(found))
	}
}

func TestTaskRepository_FindAll_Pagination(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := make([]models.Task, 5)
	for i := range tasks {
		tasks[i] = makeTask(projectId, "task", time.Duration(i+1)*100*time.Millisecond, now.Add(time.Duration(i)*time.Minute))
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	page1, total, err := TaskRepository.FindAll(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 2, "recorded_at")
	if err != nil {
		t.Fatalf("FindAll page 1 failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(page1))
	}

	page3, _, err := TaskRepository.FindAll(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 3, 2, "recorded_at")
	if err != nil {
		t.Fatalf("FindAll page 3 failed: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("expected 1 result on page 3, got %d", len(page3))
	}
}

func TestTaskRepository_FindGroupedByTaskName(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "email.send", 100*time.Millisecond, now),
		makeTask(projectId, "email.send", 300*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "report.generate", 500*time.Millisecond, now.Add(2*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	stats, total, err := TaskRepository.FindGroupedByTaskName(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "")
	if err != nil {
		t.Fatalf("FindGroupedByTaskName failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 distinct task names, got %d", total)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 grouped stats, got %d", len(stats))
	}

	// First should be email.send (count=2) when ordered by count desc
	if stats[0].TaskName != "email.send" {
		t.Errorf("expected first group to be 'email.send', got %q", stats[0].TaskName)
	}
	if stats[0].Count != 2 {
		t.Errorf("expected count 2 for email.send, got %d", stats[0].Count)
	}
}

func TestTaskRepository_FindGroupedByTaskName_RootFilterAndSearch(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	rootTask := makeTask(projectId, "scheduler.cleanup", 100*time.Millisecond, now)
	rootTask.IsRoot = true
	nonRootTask := makeTask(projectId, "queue.process", 200*time.Millisecond, now.Add(time.Minute))
	nonRootTask.IsRoot = false

	if err := TaskRepository.InsertAsync(ctx, []models.Task{rootTask, nonRootTask}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	// rootFilter=root → only the cleanup task
	stats, _, err := TaskRepository.FindGroupedByTaskName(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "root")
	if err != nil {
		t.Fatalf("FindGroupedByTaskName(root) failed: %v", err)
	}
	if len(stats) != 1 || stats[0].TaskName != "scheduler.cleanup" {
		t.Errorf("rootFilter=root: expected 1 stats row 'scheduler.cleanup', got %+v", stats)
	}
	if !stats[0].HasRoot || stats[0].HasNonRoot {
		t.Errorf("expected HasRoot=true, HasNonRoot=false, got %+v", stats[0])
	}

	// rootFilter=non_root → only the queue.process task
	stats, _, err = TaskRepository.FindGroupedByTaskName(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "non_root")
	if err != nil {
		t.Fatalf("FindGroupedByTaskName(non_root) failed: %v", err)
	}
	if len(stats) != 1 || stats[0].TaskName != "queue.process" {
		t.Errorf("rootFilter=non_root: expected 1 stats row 'queue.process', got %+v", stats)
	}
	if stats[0].HasRoot || !stats[0].HasNonRoot {
		t.Errorf("expected HasRoot=false, HasNonRoot=true, got %+v", stats[0])
	}

	// search=queue → only matching task
	stats, _, err = TaskRepository.FindGroupedByTaskName(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "queue", "")
	if err != nil {
		t.Fatalf("FindGroupedByTaskName(search) failed: %v", err)
	}
	if len(stats) != 1 || stats[0].TaskName != "queue.process" {
		t.Errorf("search=queue: expected 1 stats row 'queue.process', got %+v", stats)
	}
}

func TestTaskRepository_FindByTaskName(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "email.send", 100*time.Millisecond, now),
		makeTask(projectId, "email.send", 200*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "other.task", 300*time.Millisecond, now.Add(2*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, total, err := TaskRepository.FindByTaskName(ctx, projectId, "email.send", now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "recorded_at", "desc")
	if err != nil {
		t.Fatalf("FindByTaskName failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 results, got %d", len(found))
	}
	for _, f := range found {
		if f.TaskName != "email.send" {
			t.Errorf("expected task name 'email.send', got %q", f.TaskName)
		}
	}
}

func TestTaskRepository_FindById(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	task := makeTask(projectId, "specific.task", 150*time.Millisecond, now)

	if err := TaskRepository.InsertAsync(ctx, []models.Task{task}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := TaskRepository.FindById(ctx, projectId, task.Id, nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find task, got nil")
	}
	if found.TaskName != "specific.task" {
		t.Errorf("expected task name 'specific.task', got %q", found.TaskName)
	}
	if found.Duration != 150*time.Millisecond {
		t.Errorf("expected duration 150ms, got %v", found.Duration)
	}
}

func TestTaskRepository_FindById_NotFound(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	found, err := TaskRepository.FindById(ctx, uuid.New(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for unknown task, got %+v", found)
	}
}

func TestTaskRepository_CountByHour(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC()).Truncate(time.Hour)

	tasks := []models.Task{
		makeTask(projectId, "task-a", 100*time.Millisecond, now),
		makeTask(projectId, "task-b", 100*time.Millisecond, now.Add(30*time.Minute)),
		makeTask(projectId, "task-c", 100*time.Millisecond, now.Add(time.Hour+10*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	points, err := TaskRepository.CountByHour(ctx, projectId, now.Add(-time.Minute), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("CountByHour failed: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 hourly buckets, got %d", len(points))
	}

	if points[0].Value != 2 {
		t.Errorf("expected 2 tasks in first hour, got %v", points[0].Value)
	}
	if points[1].Value != 1 {
		t.Errorf("expected 1 task in second hour, got %v", points[1].Value)
	}
}

func TestTaskRepository_CountByInterval(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tasks := []models.Task{
		makeTask(projectId, "task-1", 100*time.Millisecond, base),
		makeTask(projectId, "task-2", 100*time.Millisecond, base.Add(10*time.Minute)),
		makeTask(projectId, "task-3", 100*time.Millisecond, base.Add(35*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	points, err := TaskRepository.CountByInterval(ctx, projectId, base.Add(-time.Minute), base.Add(time.Hour), 30)
	if err != nil {
		t.Fatalf("CountByInterval failed: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 interval buckets, got %d", len(points))
	}
}

func TestTaskRepository_FindWorstTasks(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "fast.task", 10*time.Millisecond, now),
		makeTask(projectId, "fast.task", 20*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "slow.task", 1*time.Second, now),
		makeTask(projectId, "slow.task", 5*time.Second, now.Add(time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	worst, err := TaskRepository.FindWorstTasks(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 5)
	if err != nil {
		t.Fatalf("FindWorstTasks failed: %v", err)
	}

	if len(worst) != 2 {
		t.Fatalf("expected 2 task groups, got %d", len(worst))
	}

	// slow.task should have higher impact (larger P95-P50 spread * count)
	if worst[0].TaskName != "slow.task" {
		t.Errorf("expected worst task to be 'slow.task', got %q", worst[0].TaskName)
	}
}

func TestTaskRepository_GetTaskStats(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	tasks := []models.Task{
		makeTask(projectId, "measured.task", 100*time.Millisecond, now),
		makeTask(projectId, "measured.task", 200*time.Millisecond, now.Add(time.Minute)),
		makeTask(projectId, "measured.task", 300*time.Millisecond, now.Add(2*time.Minute)),
	}

	if err := TaskRepository.InsertAsync(ctx, tasks); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	stats, err := TaskRepository.GetTaskStats(ctx, projectId, "measured.task", start, end)
	if err != nil {
		t.Fatalf("GetTaskStats failed: %v", err)
	}

	if stats.Count != 3 {
		t.Errorf("expected count 3, got %d", stats.Count)
	}

	// avg = 200ms
	assertApproxEqual(t, "AvgDuration", stats.AvgDuration, 200.0, 1.0)

	// median of [100, 200, 300] = 200ms
	assertApproxEqual(t, "MedianDuration", stats.MedianDuration, 200.0, 1.0)

	if stats.P95Duration < 200 || stats.P95Duration > 300 {
		t.Errorf("P95Duration %v out of expected range [200, 300]ms", stats.P95Duration)
	}

	if stats.Throughput <= 0 {
		t.Errorf("expected positive throughput, got %v", stats.Throughput)
	}
}

func TestTaskRepository_InsertEmpty(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	err := TaskRepository.InsertAsync(ctx, []models.Task{})
	if err != nil {
		t.Fatalf("InsertAsync with empty slice should not error: %v", err)
	}
}
