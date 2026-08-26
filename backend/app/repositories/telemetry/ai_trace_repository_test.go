package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestAiTraceRepository_GetTraceNameStats(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeAiTrace(projectId, "summarize", 100*time.Millisecond, 100, 0.5, now),
		makeAiTrace(projectId, "summarize", 200*time.Millisecond, 200, 1.0, now.Add(time.Minute)),
		makeAiTrace(projectId, "summarize", 300*time.Millisecond, 300, 1.5, now.Add(2*time.Minute)),
	}

	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	stats, err := AiTraceRepository.GetTraceNameStats(ctx, projectId, "summarize", start, end)
	if err != nil {
		t.Fatalf("GetTraceNameStats failed: %v", err)
	}

	if stats.Count != 3 {
		t.Errorf("expected count 3, got %d", stats.Count)
	}
	if stats.TotalTokens != 600 {
		t.Errorf("expected total tokens 600, got %d", stats.TotalTokens)
	}
	assertApproxEqual(t, "TotalCost", stats.TotalCost, 3.0, 0.001)
	assertApproxEqual(t, "AvgDuration", stats.AvgDuration, 200.0, 1.0)
	assertApproxEqual(t, "MedianDuration", stats.MedianDuration, 200.0, 1.0)
	assertApproxEqual(t, "AvgInputTokens", stats.AvgInputTokens, 100.0, 0.001)
}

func TestAiTraceRepository_GetTraceNameStats_EmptyWindow(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	stats, err := AiTraceRepository.GetTraceNameStats(ctx, uuid.New(), "missing", now.Add(-time.Hour), now)
	if err != nil {
		t.Fatalf("GetTraceNameStats failed: %v", err)
	}
	if stats.Count != 0 {
		t.Errorf("expected count 0, got %d", stats.Count)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("expected total tokens 0, got %d", stats.TotalTokens)
	}
}

func makeConversationTrace(projectId uuid.UUID, conversationId, userId, model string, totalCost float64, toolNames []string, flaggedTerms []string, recordedAt time.Time) models.AiTrace {
	trace := makeAiTrace(projectId, "agent", 100*time.Millisecond, 100, totalCost, recordedAt)
	trace.ConversationId = conversationId
	trace.UserId = userId
	trace.Model = model
	trace.ToolCallCount = int64(len(toolNames))
	trace.ToolNames = toolNames
	trace.Flagged = len(flaggedTerms) > 0
	trace.FlaggedTerms = flaggedTerms
	return trace
}

func TestAiTraceRepository_ConversationFieldsRoundTrip(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	trace := makeConversationTrace(projectId, "conv-1", "user-1", "gpt-4o", 0.5, []string{"get_weather", "get_time"}, []string{"bullshit"}, now)
	if err := AiTraceRepository.InsertAsync(ctx, []models.AiTrace{trace}); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	found, err := AiTraceRepository.FindById(ctx, projectId, trace.Id, nil)
	if err != nil {
		t.Fatalf("FindById failed: %v", err)
	}
	if found == nil {
		t.Fatal("trace not found after insert")
	}
	if found.ConversationId != "conv-1" {
		t.Errorf("ConversationId = %q, expected conv-1", found.ConversationId)
	}
	if found.ToolCallCount != 2 {
		t.Errorf("ToolCallCount = %d, expected 2", found.ToolCallCount)
	}
	if len(found.ToolNames) != 2 || found.ToolNames[0] != "get_weather" || found.ToolNames[1] != "get_time" {
		t.Errorf("ToolNames = %v, expected [get_weather get_time]", found.ToolNames)
	}
	if !found.Flagged {
		t.Error("Flagged = false, expected true")
	}
	if len(found.FlaggedTerms) != 1 || found.FlaggedTerms[0] != "bullshit" {
		t.Errorf("FlaggedTerms = %v, expected [bullshit]", found.FlaggedTerms)
	}
}

func TestAiTraceRepository_FindConversations(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, []string{"get_weather"}, nil, now),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o-mini", 2.0, []string{"get_time"}, []string{"damn"}, now.Add(time.Minute)),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 3.0, []string{"get_weather"}, nil, now.Add(2*time.Minute)),
		makeConversationTrace(projectId, "conv-b", "user-2", "gpt-4o", 0.5, nil, nil, now),
		// Legacy row without a conversation id must be excluded from grouping.
		makeConversationTrace(projectId, "", "user-3", "gpt-4o", 9.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from, to := now.Add(-time.Hour), now.Add(time.Hour)
	stats, total, thresholds, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "total_cost", "desc", "", "", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 conversations, got %d", total)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(stats))
	}
	if thresholds == nil {
		t.Fatal("expected thresholds, got nil")
	}

	convA := stats[0]
	if convA.ConversationId != "conv-a" {
		t.Fatalf("expected conv-a first when sorted by cost, got %q", convA.ConversationId)
	}
	if convA.Turns != 3 {
		t.Errorf("conv-a turns = %d, expected 3", convA.Turns)
	}
	assertApproxEqual(t, "conv-a TotalCost", convA.TotalCost, 6.0, 0.001)
	if convA.UserId != "user-1" {
		t.Errorf("conv-a userId = %q, expected user-1", convA.UserId)
	}
	if convA.ToolCallCount != 3 {
		t.Errorf("conv-a toolCallCount = %d, expected 3", convA.ToolCallCount)
	}
	if len(convA.ToolNames) != 2 {
		t.Errorf("conv-a toolNames = %v, expected union of 2 names", convA.ToolNames)
	}
	if len(convA.Models) != 2 {
		t.Errorf("conv-a models = %v, expected 2 distinct models", convA.Models)
	}
	if !convA.Flagged {
		t.Error("conv-a should be flagged (one turn matched)")
	}
	if len(convA.FlaggedTerms) != 1 || convA.FlaggedTerms[0] != "damn" {
		t.Errorf("conv-a flaggedTerms = %v, expected [damn]", convA.FlaggedTerms)
	}
	if !convA.LastSeen.After(convA.FirstSeen) {
		t.Errorf("conv-a lastSeen %v should be after firstSeen %v", convA.LastSeen, convA.FirstSeen)
	}

	if stats[1].ConversationId != "conv-b" || stats[1].Turns != 1 {
		t.Errorf("conv-b row wrong: %+v", stats[1])
	}
	if stats[1].Flagged {
		t.Error("conv-b should not be flagged")
	}
}

func TestAiTraceRepository_FindConversations_Filters(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, nil, []string{"crap"}, now),
		makeConversationTrace(projectId, "conv-b", "user-2", "claude-sonnet-5", 2.0, nil, nil, now),
		makeConversationTrace(projectId, "conv-c", "user-2", "gpt-4o", 3.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from, to := now.Add(-time.Hour), now.Add(time.Hour)

	flagged, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "", "", "", "", true)
	if err != nil {
		t.Fatalf("FindConversations flaggedOnly failed: %v", err)
	}
	if total != 1 || len(flagged) != 1 || flagged[0].ConversationId != "conv-a" {
		t.Errorf("flaggedOnly expected only conv-a, got total=%d rows=%+v", total, flagged)
	}

	byUser, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "", "user-2", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations userId failed: %v", err)
	}
	if total != 2 || len(byUser) != 2 {
		t.Errorf("userId filter expected 2 conversations, got total=%d", total)
	}

	byModel, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "", "", "claude-sonnet-5", "", false)
	if err != nil {
		t.Fatalf("FindConversations model failed: %v", err)
	}
	if total != 1 || len(byModel) != 1 || byModel[0].ConversationId != "conv-b" {
		t.Errorf("model filter expected conv-b, got total=%d rows=%+v", total, byModel)
	}

	bySearch, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "CONV-C", "", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations search failed: %v", err)
	}
	if total != 1 || len(bySearch) != 1 || bySearch[0].ConversationId != "conv-c" {
		t.Errorf("search expected conv-c, got total=%d rows=%+v", total, bySearch)
	}

	paged, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 2, 2, "total_cost", "desc", "", "", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations pagination failed: %v", err)
	}
	if total != 3 || len(paged) != 1 {
		t.Errorf("page 2 of size 2 expected 1 row of 3 total, got total=%d rows=%d", total, len(paged))
	}
}

func TestAiTraceRepository_FindConversations_ToolFilterAndSearch(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		// conv-a: 3 turns, only the middle one used a tool.
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 2.0, []string{"ask_math_agent", "calculate"}, nil, now.Add(time.Minute)),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 3.0, nil, nil, now.Add(2*time.Minute)),
		// conv-b: uses get_weather; name contains "calc" nowhere.
		makeConversationTrace(projectId, "conv-b", "user-2", "gpt-4o", 0.5, []string{"get_weather"}, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from, to := now.Add(-time.Hour), now.Add(time.Hour)

	// Tool filter matches a single turn but returns the whole conversation's
	// aggregates.
	byTool, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "", "", "", "calculate", false)
	if err != nil {
		t.Fatalf("FindConversations toolName failed: %v", err)
	}
	if total != 1 || len(byTool) != 1 || byTool[0].ConversationId != "conv-a" {
		t.Fatalf("toolName filter expected conv-a, got total=%d rows=%+v", total, byTool)
	}
	if byTool[0].Turns != 3 {
		t.Errorf("tool-filtered conversation should keep all 3 turns, got %d", byTool[0].Turns)
	}
	assertApproxEqual(t, "tool-filtered TotalCost", byTool[0].TotalCost, 6.0, 0.001)

	// A partial tool name must not match as an exact tool filter...
	byPartialTool, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "", "", "", "calc", false)
	if err != nil {
		t.Fatalf("FindConversations partial toolName failed: %v", err)
	}
	if total != 0 || len(byPartialTool) != 0 {
		t.Errorf("partial tool name should not match exact filter, got total=%d", total)
	}

	// ...but free-text search covers tool names (and models) as substrings.
	bySearch, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "math", "", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations search failed: %v", err)
	}
	if total != 1 || len(bySearch) != 1 || bySearch[0].ConversationId != "conv-a" {
		t.Errorf("search 'math' expected conv-a via tool name, got total=%d rows=%+v", total, bySearch)
	}
	if len(bySearch) == 1 && bySearch[0].Turns != 3 {
		t.Errorf("searched conversation should keep all 3 turns, got %d", bySearch[0].Turns)
	}

	byModelSearch, total, _, err := AiTraceRepository.FindConversations(ctx, projectId, from, to, 1, 50, "last_seen", "desc", "GPT-4O", "", "", "", false)
	if err != nil {
		t.Fatalf("FindConversations model search failed: %v", err)
	}
	if total != 2 {
		t.Errorf("search 'GPT-4O' expected both conversations, got total=%d rows=%+v", total, byModelSearch)
	}
}

func TestAiTraceRepository_ListToolNames(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, []string{"get_weather", "calculate"}, nil, now),
		makeConversationTrace(projectId, "conv-b", "user-2", "gpt-4o", 1.0, []string{"get_weather"}, nil, now),
		makeConversationTrace(projectId, "conv-c", "user-2", "gpt-4o", 1.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	tools, err := AiTraceRepository.ListToolNames(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListToolNames failed: %v", err)
	}
	if len(tools) != 2 || tools[0] != "calculate" || tools[1] != "get_weather" {
		t.Errorf("ListToolNames = %v, expected [calculate get_weather]", tools)
	}
}

func TestAiTraceRepository_FindByConversationId(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 2.0, []string{"get_time"}, nil, now.Add(time.Minute)),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, []string{"get_weather"}, []string{"damn"}, now),
		makeConversationTrace(projectId, "conv-other", "user-2", "gpt-4o", 5.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	turns, stats, err := AiTraceRepository.FindByConversationId(ctx, projectId, "conv-a", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("FindByConversationId failed: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if !turns[0].RecordedAt.Before(turns[1].RecordedAt) {
		t.Error("turns should be ordered by recorded_at ASC")
	}
	if stats.Turns != 2 {
		t.Errorf("stats.Turns = %d, expected 2", stats.Turns)
	}
	assertApproxEqual(t, "stats.TotalCost", stats.TotalCost, 3.0, 0.001)
	if stats.ToolCallCount != 2 {
		t.Errorf("stats.ToolCallCount = %d, expected 2", stats.ToolCallCount)
	}
	if !stats.Flagged || len(stats.FlaggedTerms) != 1 {
		t.Errorf("stats should be flagged with one term, got flagged=%v terms=%v", stats.Flagged, stats.FlaggedTerms)
	}
	if stats.UserId != "user-1" {
		t.Errorf("stats.UserId = %q, expected user-1", stats.UserId)
	}

	empty, _, err := AiTraceRepository.FindByConversationId(ctx, projectId, "missing", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("FindByConversationId for missing id failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected no turns for missing conversation, got %d", len(empty))
	}
}

func TestAiTraceRepository_FindUserStats(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		// user-1: conversations of 1, 2 and 5 turns (median 2, min 1, avg 8/3).
		makeConversationTrace(projectId, "u1-c1", "user-1", "gpt-4o", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "u1-c2", "user-1", "gpt-4o", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "u1-c2", "user-1", "gpt-4o", 1.0, nil, []string{"crap"}, now.Add(time.Minute)),
		makeConversationTrace(projectId, "u1-c3", "user-1", "gpt-4o", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "u1-c3", "user-1", "gpt-4o", 1.0, nil, nil, now.Add(time.Minute)),
		makeConversationTrace(projectId, "u1-c3", "user-1", "gpt-4o", 1.0, nil, nil, now.Add(2*time.Minute)),
		makeConversationTrace(projectId, "u1-c3", "user-1", "gpt-4o", 1.0, nil, nil, now.Add(3*time.Minute)),
		makeConversationTrace(projectId, "u1-c3", "user-1", "gpt-4o", 1.0, nil, nil, now.Add(4*time.Minute)),
		// user-2: one conversation of 2 turns costing 4.0 total.
		makeConversationTrace(projectId, "u2-c1", "user-2", "gpt-4o", 3.0, nil, nil, now),
		makeConversationTrace(projectId, "u2-c1", "user-2", "gpt-4o", 1.0, nil, nil, now.Add(time.Minute)),
		// Rows without user or conversation ids are excluded.
		makeConversationTrace(projectId, "anon-c1", "", "gpt-4o", 9.0, nil, nil, now),
		makeConversationTrace(projectId, "", "user-9", "gpt-4o", 9.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	from, to := now.Add(-time.Hour), now.Add(time.Hour)
	stats, total, err := AiTraceRepository.FindUserStats(ctx, projectId, from, to, 1, 50, "total_cost", "desc", "")
	if err != nil {
		t.Fatalf("FindUserStats failed: %v", err)
	}
	if total != 2 || len(stats) != 2 {
		t.Fatalf("expected 2 users, got total=%d rows=%d", total, len(stats))
	}

	byUser := map[string]models.AiUserStats{}
	for _, s := range stats {
		byUser[s.UserId] = s
	}

	u1 := byUser["user-1"]
	if u1.ConversationCount != 3 {
		t.Errorf("user-1 conversations = %d, expected 3", u1.ConversationCount)
	}
	if u1.TotalCalls != 8 {
		t.Errorf("user-1 calls = %d, expected 8", u1.TotalCalls)
	}
	if u1.MinTurns != 1 {
		t.Errorf("user-1 minTurns = %d, expected 1", u1.MinTurns)
	}
	assertApproxEqual(t, "user-1 medianTurns", u1.MedianTurns, 2.0, 0.001)
	assertApproxEqual(t, "user-1 avgTurns", u1.AvgTurns, 8.0/3.0, 0.001)
	assertApproxEqual(t, "user-1 totalCost", u1.TotalCost, 8.0, 0.001)
	assertApproxEqual(t, "user-1 avgConversationCost", u1.AvgCostPerConversation, 8.0/3.0, 0.001)
	if u1.FlaggedConversationCount != 1 {
		t.Errorf("user-1 flaggedConversations = %d, expected 1", u1.FlaggedConversationCount)
	}

	u2 := byUser["user-2"]
	if u2.ConversationCount != 1 || u2.TotalCalls != 2 {
		t.Errorf("user-2 stats wrong: %+v", u2)
	}
	assertApproxEqual(t, "user-2 medianTurns", u2.MedianTurns, 2.0, 0.001)
	assertApproxEqual(t, "user-2 totalCost", u2.TotalCost, 4.0, 0.001)
	if u2.FlaggedConversationCount != 0 {
		t.Errorf("user-2 flaggedConversations = %d, expected 0", u2.FlaggedConversationCount)
	}

	// stats sorted by total_cost desc: user-1 (8.0) before user-2 (4.0).
	if stats[0].UserId != "user-1" {
		t.Errorf("expected user-1 first by total cost, got %q", stats[0].UserId)
	}
}

func TestAiTraceRepository_GetConversationCosts(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.5, nil, nil, now),
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 2.5, nil, nil, now.Add(time.Minute)),
		makeConversationTrace(projectId, "conv-b", "user-2", "gpt-4o", 0.25, nil, nil, now),
		// Outside the lookback window.
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 100.0, nil, nil, now.Add(-48*time.Hour)),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	costs, err := AiTraceRepository.GetConversationCosts(ctx, projectId, []string{"conv-a", "conv-b", "conv-missing"}, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetConversationCosts failed: %v", err)
	}
	assertApproxEqual(t, "conv-a cost", costs["conv-a"], 4.0, 0.001)
	assertApproxEqual(t, "conv-b cost", costs["conv-b"], 0.25, 0.001)
	if _, ok := costs["conv-missing"]; ok {
		t.Error("conv-missing should not be present in the cost map")
	}

	empty, err := AiTraceRepository.GetConversationCosts(ctx, projectId, nil, now)
	if err != nil {
		t.Fatalf("GetConversationCosts with no ids failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty map, got %v", empty)
	}
}

func TestAiTraceRepository_ListModels(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	traces := []models.AiTrace{
		makeConversationTrace(projectId, "conv-a", "user-1", "gpt-4o", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "conv-b", "user-1", "claude-sonnet-5", 1.0, nil, nil, now),
		makeConversationTrace(projectId, "conv-c", "user-1", "gpt-4o", 1.0, nil, nil, now),
	}
	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	names, err := AiTraceRepository.ListModels(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(names) != 2 || names[0] != "claude-sonnet-5" || names[1] != "gpt-4o" {
		t.Errorf("ListModels = %v, expected [claude-sonnet-5 gpt-4o]", names)
	}
}

// Percentiles must describe the rows the filter selects, since the list is
// ordered and paginated on them.
func TestAiTraceRepository_FindGroupedByTraceName_PercentilesUseFilteredRows(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	var traces []models.AiTrace
	for range 4 {
		traces = append(traces, makeAiTraceWithRoot(projectId, "agent.plan", 200*time.Millisecond, 100, 0.01, now, true))
		traces = append(traces, makeAiTraceWithRoot(projectId, "agent.plan", 30*time.Second, 100, 0.01, now, false))
	}

	if err := AiTraceRepository.InsertAsync(ctx, traces); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	stats, _, err := AiTraceRepository.FindGroupedByTraceName(ctx, projectId, now.Add(-time.Hour), now.Add(time.Hour), 1, 10, "count", "desc", "", "root")
	if err != nil {
		t.Fatalf("FindGroupedByTraceName failed: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 grouped stat, got %d", len(stats))
	}
	if stats[0].Count != 4 {
		t.Errorf("expected count 4 under rootFilter=root, got %d", stats[0].Count)
	}
	if stats[0].P50Duration != 200*time.Millisecond {
		t.Errorf("P50 computed over unfiltered rows: got %v, want 200ms", stats[0].P50Duration)
	}
	if stats[0].P95Duration != 200*time.Millisecond {
		t.Errorf("P95 computed over unfiltered rows: got %v, want 200ms", stats[0].P95Duration)
	}
}
