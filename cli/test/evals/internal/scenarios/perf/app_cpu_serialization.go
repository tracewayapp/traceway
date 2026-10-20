package perf

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/app-cpu-serialization", papSeedCPUSerialization) }

// papSeedCPUSerialization: the transaction export reads the rows quickly,
// then spends 0.3-7s building the JSON body in memory (a gap between the
// SELECT and the audit INSERT, with no span) before writing it out in a few
// milliseconds. CPU jumps while an export serializes. Distractor: the
// bodies are large (up to ~4MB, logged per export), but the time is spent
// before a single byte is sent, and the cost per row (~0.35ms) is CPU, not
// transfer.
func papSeedCPUSerialization(s *seed.Scenario) error {
	p := s.Project("ledger", seed.OTel)
	api := p.Service("accounts-api", seed.DotNet()).Versions("4.2.0")
	window := 7 * 24 * time.Hour

	var busy []papInterval
	var sorted bool
	serOf := map[*seed.Request]papInterval{}
	export := api.Endpoint("GET", "/api/accounts/{accountId}/transactions/export")
	export.Traffic(seed.Daily(160), seed.Diurnal(), seed.Over(window))
	export.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(1.2, 2.8)) // JWT validation, model binding
		account := 10000 + r.IntN(80000)
		r.DB("SELECT id, number, currency, owner_id FROM accounts WHERE id = @AccountId", r.Ms(1.2, 3))
		rows := int(math.Min(20000, math.Max(300, float64(r.LogNormal(3400*time.Millisecond, 16000*time.Millisecond).Milliseconds()))))
		r.DB("SELECT t.id, t.posted_at, t.amount, t.currency, t.counterparty, t.reference, t.category, t.balance_after, t.metadata FROM transactions t WHERE t.account_id = @AccountId AND t.posted_at >= @From AND t.posted_at < @To ORDER BY t.posted_at",
			time.Duration(float64(rows)*0.009*(0.85+0.3*r.Float())*float64(time.Millisecond))+r.Ms(6, 14))
		ser := time.Duration(float64(rows) * 0.35 * (0.9 + 0.2*r.Float()) * float64(time.Millisecond))
		at := r.At()
		r.Gap(ser)
		busy = append(busy, papInterval{at, at.Add(ser)})
		serOf[r.Request()] = papInterval{at, at.Add(ser)}
		sorted = false
		bytes := rows*(205+r.IntN(30)) + 2048
		r.DB("INSERT INTO export_audit (account_id, requested_by, row_count, byte_count, created_at) VALUES (@AccountId, @UserId, @RowCount, @ByteCount, now())", r.Ms(1.5, 4))
		r.Log(seed.SevInfo, fmt.Sprintf("Exported %d transactions (%d bytes) for account %d", rows, bytes, account),
			seed.A("RowCount", rows), seed.A("ByteCount", bytes), seed.A("AccountId", account),
			seed.A("category.name", "Accounts.Api.Exports.TransactionExportEndpoints"))
		r.Gap(time.Duration(float64(bytes)/1e9*float64(time.Second)) + r.Ms(2, 6)) // write the body to Kestrel
	})

	page := api.Endpoint("GET", "/api/accounts/{accountId}/transactions")
	page.Traffic(seed.Daily(600), seed.Diurnal(), seed.Over(window))
	page.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(1.2, 2.8))
		r.DB("SELECT t.id, t.posted_at, t.amount, t.currency, t.counterparty, t.reference, t.category, t.balance_after, t.metadata FROM transactions t WHERE t.account_id = @AccountId ORDER BY t.posted_at DESC LIMIT @PageSize OFFSET @Offset",
			r.Ms(3, 9))
		r.Gap(r.Ms(1.5, 3.5)) // 50 rows through a cached JsonSerializerOptions
	})

	acct := api.Endpoint("GET", "/api/accounts/{accountId}")
	acct.Traffic(seed.Daily(450), seed.Diurnal(), seed.Over(window))
	acct.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(1.2, 2.8))
		r.DB("SELECT id, number, currency, owner_id, balance, updated_at FROM accounts WHERE id = @AccountId", r.Ms(1.2, 3))
		r.Gap(r.Ms(0.3, 0.8))
	})

	s.Background(api, seed.TypicalTraffic(6))

	// Two vCPUs; a serializing export keeps one busy.
	serLoad := func(t time.Time) float64 {
		if !sorted {
			sort.Slice(busy, func(i, j int) bool { return busy[i].from.Before(busy[j].from) })
			sorted = true
		}
		return 0.48 * float64(papActive(busy, t, 30*time.Second))
	}
	api.Host().CPU(seed.Plus(seed.Flat(0.12, 0.2), serLoad))
	api.Host().Memory(seed.Flat(0.46, 0.55))

	// The host metrics are one sample a minute; expose the slowest export of
	// the day whose serialization spans a sample, so its CPU is on record.
	var sampled *seed.Request
	slowSampled := func() *seed.Request {
		if sampled != nil {
			return sampled
		}
		for _, rq := range export.Requests() {
			iv := serOf[rq]
			if rq.Start.Before(s.Ago(24*time.Hour)) || iv.from.Truncate(time.Minute).Equal(iv.to.Truncate(time.Minute)) {
				continue
			}
			if sampled == nil || rq.Duration() > sampled.Duration() {
				sampled = rq
			}
		}
		sampled.KeepDetail()
		return sampled
	}
	s.Expose("slowRequest", func() any { return slowSampled().Ref() })
	s.Expose("smallRequest", export.Pick(func(r *seed.Request) bool {
		return r.Start.After(s.Ago(24*time.Hour)) && r.Duration() > 500*time.Millisecond && r.Duration() < 1200*time.Millisecond
	}))
	s.Expose("slowFrom", func() any {
		return slowSampled().Start.Add(-10 * time.Minute).Truncate(time.Minute).Format(time.RFC3339)
	})
	s.Expose("slowTo", func() any {
		return slowSampled().Start.Add(11 * time.Minute).Truncate(time.Minute).Format(time.RFC3339)
	})
	s.Expose("p50Ms24h", func() any { return export.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return export.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}
