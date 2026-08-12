package shared

import (
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

// JoinCSV renders a string slice as the comma-separated form the ai_traces
// tool_names / flagged_terms columns use. Values are sanitized at ingest so
// they never contain commas.
func JoinCSV(values []string) string {
	return strings.Join(values, ",")
}

// SplitCSV splits a comma-separated column value, dropping empty entries.
func SplitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// UnionCSV splits a string-agg of comma-separated column values (each element
// itself possibly a comma-separated list) into a sorted, deduplicated slice.
func UnionCSV(concatenated string) []string {
	parts := SplitCSV(concatenated)
	if len(parts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ConversationThresholds computes the range-wide P95 outlier cutoffs over the
// full (pre-pagination) conversation group list. Used by the embedded
// backends; ClickHouse computes the same values in SQL.
func ConversationThresholds(stats []models.AiConversationStats) *models.AiConversationThresholds {
	if len(stats) == 0 {
		return &models.AiConversationThresholds{}
	}
	costs := make([]float64, len(stats))
	turns := make([]float64, len(stats))
	for i, s := range stats {
		costs[i] = s.TotalCost
		turns[i] = float64(s.Turns)
	}
	sort.Float64s(costs)
	sort.Float64s(turns)
	return &models.AiConversationThresholds{
		P95Cost:  ComputePercentile(costs, 0.95),
		P95Turns: ComputePercentile(turns, 0.95),
	}
}

// SortAndPageConversations orders conversation stats by a whitelisted column
// and clips to the requested page. Shared by the embedded backends, which
// fetch all groups and sort in Go.
func SortAndPageConversations(stats []models.AiConversationStats, orderBy, sortDirection string, page, pageSize int) []models.AiConversationStats {
	orderByMap := map[string]func(i, j int) bool{
		"turns":           func(i, j int) bool { return stats[i].Turns > stats[j].Turns },
		"total_cost":      func(i, j int) bool { return stats[i].TotalCost > stats[j].TotalCost },
		"total_tokens":    func(i, j int) bool { return stats[i].TotalTokens > stats[j].TotalTokens },
		"tool_call_count": func(i, j int) bool { return stats[i].ToolCallCount > stats[j].ToolCallCount },
		"user_id":         func(i, j int) bool { return stats[i].UserId > stats[j].UserId },
		"first_seen":      func(i, j int) bool { return stats[i].FirstSeen.After(stats[j].FirstSeen) },
		"last_seen":       func(i, j int) bool { return stats[i].LastSeen.After(stats[j].LastSeen) },
	}
	sortFn, ok := orderByMap[orderBy]
	if !ok {
		sortFn = orderByMap["last_seen"]
	}
	if sortDirection == "asc" {
		origFn := sortFn
		sortFn = func(i, j int) bool { return !origFn(i, j) }
	}
	sort.SliceStable(stats, sortFn)
	return PageSlice(stats, page, pageSize)
}

// UserConversationAgg is one (user, conversation) group produced by the inner
// aggregation query; AggregateUserStats rolls these up per user.
type UserConversationAgg struct {
	UserId   string
	Turns    int64
	Cost     float64
	Tokens   int64
	Flagged  bool
	LastSeen time.Time
}

// AggregateUserStats computes per-user conversation statistics (count,
// avg/min/median turns, cost aggregates) from per-conversation rows. Used by
// the embedded backends, which have no SQL median; ClickHouse and DuckDB
// could compute this in SQL but share the Go path via their inner queries to
// keep the three backends' semantics identical.
func AggregateUserStats(rows []UserConversationAgg) []models.AiUserStats {
	type userAcc struct {
		turns    []float64
		calls    int64
		cost     float64
		tokens   int64
		flagged  int64
		minTurns int64
		lastSeen time.Time
	}
	byUser := map[string]*userAcc{}
	order := []string{}
	for _, row := range rows {
		acc, ok := byUser[row.UserId]
		if !ok {
			acc = &userAcc{minTurns: row.Turns}
			byUser[row.UserId] = acc
			order = append(order, row.UserId)
		}
		acc.turns = append(acc.turns, float64(row.Turns))
		acc.calls += row.Turns
		acc.cost += row.Cost
		acc.tokens += row.Tokens
		if row.Flagged {
			acc.flagged++
		}
		if row.Turns < acc.minTurns {
			acc.minTurns = row.Turns
		}
		if row.LastSeen.After(acc.lastSeen) {
			acc.lastSeen = row.LastSeen
		}
	}

	stats := make([]models.AiUserStats, 0, len(order))
	for _, userId := range order {
		acc := byUser[userId]
		sort.Float64s(acc.turns)
		conversations := int64(len(acc.turns))
		var avgTurns, avgCost float64
		if conversations > 0 {
			avgTurns = float64(acc.calls) / float64(conversations)
			avgCost = acc.cost / float64(conversations)
		}
		stats = append(stats, models.AiUserStats{
			UserId:                   userId,
			ConversationCount:        conversations,
			TotalCalls:               acc.calls,
			AvgTurns:                 avgTurns,
			MinTurns:                 acc.minTurns,
			MedianTurns:              ComputePercentile(acc.turns, 0.5),
			AvgCostPerConversation:   avgCost,
			TotalCost:                acc.cost,
			FlaggedConversationCount: acc.flagged,
			TotalTokens:              acc.tokens,
			LastSeen:                 acc.lastSeen,
		})
	}
	return stats
}

// SortAndPageUserStats orders user stats by a whitelisted column and clips to
// the requested page.
func SortAndPageUserStats(stats []models.AiUserStats, orderBy, sortDirection string, page, pageSize int) []models.AiUserStats {
	orderByMap := map[string]func(i, j int) bool{
		"conversation_count":         func(i, j int) bool { return stats[i].ConversationCount > stats[j].ConversationCount },
		"total_calls":                func(i, j int) bool { return stats[i].TotalCalls > stats[j].TotalCalls },
		"avg_turns":                  func(i, j int) bool { return stats[i].AvgTurns > stats[j].AvgTurns },
		"min_turns":                  func(i, j int) bool { return stats[i].MinTurns > stats[j].MinTurns },
		"median_turns":               func(i, j int) bool { return stats[i].MedianTurns > stats[j].MedianTurns },
		"avg_conversation_cost":      func(i, j int) bool { return stats[i].AvgCostPerConversation > stats[j].AvgCostPerConversation },
		"total_cost":                 func(i, j int) bool { return stats[i].TotalCost > stats[j].TotalCost },
		"flagged_conversation_count": func(i, j int) bool { return stats[i].FlaggedConversationCount > stats[j].FlaggedConversationCount },
		"total_tokens":               func(i, j int) bool { return stats[i].TotalTokens > stats[j].TotalTokens },
		"last_seen":                  func(i, j int) bool { return stats[i].LastSeen.After(stats[j].LastSeen) },
	}
	sortFn, ok := orderByMap[orderBy]
	if !ok {
		sortFn = orderByMap["total_cost"]
	}
	if sortDirection == "asc" {
		origFn := sortFn
		sortFn = func(i, j int) bool { return !origFn(i, j) }
	}
	sort.SliceStable(stats, sortFn)
	return PageSlice(stats, page, pageSize)
}

// PageSlice clips a slice to one page, mirroring the offset/limit clipping
// the embedded backends do inline elsewhere.
func PageSlice[T any](items []T, page, pageSize int) []T {
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if offset > len(items) {
		return nil
	}
	if end > len(items) {
		return items[offset:]
	}
	return items[offset:end]
}

// BuildConversationDetailStats derives conversation-level stats from the
// already-fetched turns, identically on every backend.
func BuildConversationDetailStats(turns []models.AiTrace) *models.AiConversationDetailStats {
	stats := &models.AiConversationDetailStats{}
	if len(turns) == 0 {
		return stats
	}

	var totalDuration time.Duration
	modelSet := map[string]struct{}{}
	termSet := map[string]struct{}{}
	stats.FirstSeen = turns[0].RecordedAt
	stats.LastSeen = turns[0].RecordedAt
	for _, t := range turns {
		stats.Turns++
		stats.TotalTokens += t.TotalTokens
		stats.TotalCost += t.TotalCost
		stats.ToolCallCount += t.ToolCallCount
		totalDuration += t.Duration
		if t.Model != "" {
			modelSet[t.Model] = struct{}{}
		}
		if t.Flagged {
			stats.Flagged = true
		}
		for _, term := range t.FlaggedTerms {
			termSet[term] = struct{}{}
		}
		if t.UserId != "" {
			stats.UserId = t.UserId
		}
		if t.RecordedAt.Before(stats.FirstSeen) {
			stats.FirstSeen = t.RecordedAt
		}
		if t.RecordedAt.After(stats.LastSeen) {
			stats.LastSeen = t.RecordedAt
		}
	}
	stats.AvgDuration = float64(totalDuration.Nanoseconds()) / float64(len(turns)) / 1e6
	for model := range modelSet {
		stats.Models = append(stats.Models, model)
	}
	sort.Strings(stats.Models)
	for term := range termSet {
		stats.FlaggedTerms = append(stats.FlaggedTerms, term)
	}
	sort.Strings(stats.FlaggedTerms)
	return stats
}
