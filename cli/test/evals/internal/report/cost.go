package report

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// ModelsURL is OpenRouter's public model list (prices need no key).
const ModelsURL = "https://openrouter.ai/api/v1/models"

// LivePrice is a model's price in USD per million tokens, plus its
// long-prompt tier when the provider has one.
type LivePrice struct {
	catalog.Price
	Tier          *catalog.Price `json:"tier,omitempty"`
	TierMinPrompt int            `json:"tierMinPrompt,omitempty"`
	Supported     []string       `json:"supported,omitempty"`
	Source        string         `json:"source"` // live | fallback | file
}

// FetchPrices reads live prices for the given model ids. When the list is
// unreachable each model falls back to models.yaml's fallback_price.
func FetchPrices(ctx context.Context, models []*catalog.Model, fromFile string) (map[string]LivePrice, error) {
	var raw []byte
	var err error
	source := "live"
	if fromFile != "" {
		raw, err = os.ReadFile(fromFile)
		source = "file"
	} else {
		raw, err = fetch(ctx, ModelsURL)
	}
	out := map[string]LivePrice{}
	if err == nil {
		var list struct {
			Data []struct {
				ID      string `json:"id"`
				Pricing struct {
					Prompt     string `json:"prompt"`
					Completion string `json:"completion"`
					CacheRead  string `json:"input_cache_read"`
					CacheWrite string `json:"input_cache_write"`
					Overrides  []struct {
						MinPrompt  int    `json:"min_prompt_tokens"`
						Prompt     string `json:"prompt"`
						Completion string `json:"completion"`
						CacheRead  string `json:"input_cache_read"`
						CacheWrite string `json:"input_cache_write"`
					} `json:"overrides"`
				} `json:"pricing"`
				Supported []string `json:"supported_parameters"`
			} `json:"data"`
		}
		if jerr := json.Unmarshal(raw, &list); jerr == nil {
			for _, d := range list.Data {
				p := LivePrice{Price: catalog.Price{Prompt: perM(d.Pricing.Prompt), Completion: perM(d.Pricing.Completion),
					CacheRead: perM(d.Pricing.CacheRead), CacheWrite: perM(d.Pricing.CacheWrite)}, Supported: d.Supported, Source: source}
				if len(d.Pricing.Overrides) > 0 {
					o := d.Pricing.Overrides[0]
					p.Tier = &catalog.Price{Prompt: perM(o.Prompt), Completion: perM(o.Completion), CacheRead: perM(o.CacheRead), CacheWrite: perM(o.CacheWrite)}
					p.TierMinPrompt = o.MinPrompt
				}
				out[d.ID] = p
			}
		} else {
			err = jerr
		}
	}
	missing := []string{}
	for _, m := range models {
		if _, ok := out[m.ID]; !ok {
			out[m.ID] = LivePrice{Price: m.FallbackPrice, Source: "fallback"}
			missing = append(missing, m.ID)
		}
	}
	if len(missing) > 0 && err == nil {
		err = fmt.Errorf("models not in the price list (pinned id retired?): %s", strings.Join(missing, ", "))
	}
	return out, err
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func perM(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f * 1e6
}

// RunCost estimates one run's cost from a token envelope: uncached input at
// the prompt price (the cache-write price when the model caches explicitly),
// cached input at the read price, output at the completion price.
func RunCost(p LivePrice, caching string, input, output int, cachedFrac float64) float64 {
	price := p.Price
	if p.Tier != nil && p.TierMinPrompt > 0 && input/20 > p.TierMinPrompt {
		// Long agent histories cross the long-prompt tier on their last turns.
		price = *p.Tier
	}
	cached := float64(input) * cachedFrac
	uncached := float64(input) - cached
	inPrice := price.Prompt
	if caching == "anthropic" && price.CacheWrite > 0 {
		inPrice = price.CacheWrite
	}
	return (uncached*inPrice + cached*price.CacheRead + float64(output)*price.Completion) / 1e6
}

// Estimate is a cost range for a set of runs.
type Estimate struct {
	Model   string  `json:"model"`
	ModelID string  `json:"modelId"`
	Runs    int     `json:"runs"`
	Low     float64 `json:"low"`
	High    float64 `json:"high"`
	Worst   float64 `json:"worst"` // cache never hits
	Source  string  `json:"source"`
	Basis   string  `json:"basis"` // models.yaml | baseline
}

// EstimateRuns prices n runs of a model, from the baseline's measured
// averages when it has them, else from the models.yaml envelope.
func EstimateRuns(m *catalog.Model, p LivePrice, n int, base *Baseline) Estimate {
	e := Estimate{Model: m.Key, ModelID: m.ID, Runs: n, Source: p.Source, Basis: "models.yaml"}
	lo := RunCost(p, m.Caching, m.Estimate.InputTokens[0], m.Estimate.OutputTokens[0], m.Estimate.CachedFraction)
	hi := RunCost(p, m.Caching, m.Estimate.InputTokens[1], m.Estimate.OutputTokens[1], m.Estimate.CachedFraction)
	worst := RunCost(p, "none", m.Estimate.InputTokens[1], m.Estimate.OutputTokens[1], 0)
	if base != nil {
		if mu, ok := base.PerRun[m.Key]; ok && mu.Runs >= 5 {
			lo, hi = mu.CostUSD*0.8, mu.CostUSDP90*1.2
			e.Basis = "baseline"
		}
	}
	e.Low, e.High, e.Worst = lo*float64(n), hi*float64(n), worst*float64(n)
	return e
}

// SortEstimates orders estimates by model key.
func SortEstimates(es []Estimate) {
	sort.Slice(es, func(i, j int) bool { return es[i].Model < es[j].Model })
}
