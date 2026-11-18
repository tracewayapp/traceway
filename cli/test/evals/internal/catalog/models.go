package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Model is one OpenRouter model as configured in models.yaml.
type Model struct {
	// Key is the short name used on the command line (opus, astra, judge).
	Key string `yaml:"-"`
	// ID is the pinned OpenRouter model id. Never a floating alias.
	ID       string   `yaml:"id"`
	Label    string   `yaml:"label"`
	Provider Provider `yaml:"provider"`
	// Caching is "anthropic" (explicit cache_control breakpoints) or
	// "automatic" (provider-side prefix caching, keep the prefix stable).
	Caching string `yaml:"caching"`
	// ReasoningEffort is sent as reasoning.effort when the model lists
	// "reasoning" in its supported_parameters. Empty disables it.
	ReasoningEffort string `yaml:"reasoning_effort"`
	// MaxTokensPerTurn caps one completion.
	MaxTokensPerTurn int      `yaml:"max_tokens_per_turn"`
	Temperature      *float64 `yaml:"temperature"`
	// Estimate is the per-run token envelope used by the preflight until
	// baseline.json carries measured averages.
	Estimate Estimate `yaml:"estimate"`
	// FallbackPrice (USD per million tokens) is used when the live price
	// list cannot be fetched. The preflight says when it fell back.
	FallbackPrice Price `yaml:"fallback_price"`
}

// Provider is OpenRouter's provider routing block.
type Provider struct {
	Order          []string `yaml:"order" json:"order,omitempty"`
	AllowFallbacks *bool    `yaml:"allow_fallbacks" json:"allow_fallbacks,omitempty"`
}

// Estimate is a per-run token envelope.
type Estimate struct {
	InputTokens    [2]int  `yaml:"input_tokens"`
	CachedFraction float64 `yaml:"cached_fraction"`
	OutputTokens   [2]int  `yaml:"output_tokens"`
}

// Price is USD per million tokens.
type Price struct {
	Prompt     float64 `yaml:"prompt" json:"prompt"`
	Completion float64 `yaml:"completion" json:"completion"`
	CacheRead  float64 `yaml:"cache_read" json:"cacheRead"`
	CacheWrite float64 `yaml:"cache_write" json:"cacheWrite"`
}

// Models is models.yaml.
type Models struct {
	Agents map[string]*Model `yaml:"agents"`
	Judge  *Model            `yaml:"judge"`
	// Digest is the SHA-256 of the file, recorded in every results header.
	Digest string `yaml:"-"`
}

// LoadModels reads models.yaml under root.
func LoadModels(root string) (*Models, error) {
	path := filepath.Join(root, "models.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Models
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	m.Digest = hex.EncodeToString(sum[:])
	for k, a := range m.Agents {
		a.Key = k
	}
	if m.Judge != nil {
		m.Judge.Key = "judge"
	}
	return &m, m.validate()
}

func (m *Models) validate() error {
	all := []*Model{}
	for _, a := range m.Agents {
		all = append(all, a)
	}
	if m.Judge == nil {
		return fmt.Errorf("models.yaml: judge is required")
	}
	all = append(all, m.Judge)
	for _, a := range all {
		if a.ID == "" || !strings.Contains(a.ID, "/") {
			return fmt.Errorf("models.yaml: %s: id must be a provider/model OpenRouter id", a.Key)
		}
		if strings.HasPrefix(a.ID, "~") || strings.HasSuffix(a.ID, "latest") || strings.Contains(a.ID, ":") {
			return fmt.Errorf("models.yaml: %s: %q is a floating alias or variant; pin an exact id", a.Key, a.ID)
		}
		switch a.Caching {
		case "anthropic", "automatic", "none":
		default:
			return fmt.Errorf("models.yaml: %s: caching must be anthropic, automatic or none", a.Key)
		}
	}
	return nil
}

// Agent returns the agent model by key.
func (m *Models) Agent(key string) (*Model, error) {
	a, ok := m.Agents[key]
	if !ok {
		return nil, fmt.Errorf("unknown model %q (models.yaml has %s)", key, strings.Join(m.AgentKeys(), ", "))
	}
	return a, nil
}

// AgentKeys returns the configured agent keys, sorted.
func (m *Models) AgentKeys() []string {
	keys := make([]string, 0, len(m.Agents))
	for k := range m.Agents {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ParseModelList splits "opus,astra" and checks every key exists.
func (m *Models) ParseModelList(s string) ([]*Model, error) {
	var out []*Model
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		a, err := m.Agent(k)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(out, func(x *Model) bool { return x.Key == k }) {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no models selected")
	}
	return out, nil
}
