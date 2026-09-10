package agent

import (
	"sort"
	"sync"
)

var (
	registryMu           sync.RWMutex
	integrationObservers []func()
	contextProviders     = map[string]ContextProvider{}
	triggers             = map[string]TriggerSource{}
	channels             = map[string]Channel{}
	codeHosts            = map[string]CodeHost{}
	issueTrackers        = map[string]IssueTracker{}
	executors            = map[string]Executor{}
)

func RegisterContextProvider(p ContextProvider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	contextProviders[p.Kind()] = p
}

func RegisterTrigger(t TriggerSource) {
	registryMu.Lock()
	defer registryMu.Unlock()
	triggers[t.Provider()] = t
}

func RegisterChannel(c Channel) {
	registryMu.Lock()
	defer registryMu.Unlock()
	channels[c.Provider()] = c
}

func RegisterCodeHost(h CodeHost) {
	registryMu.Lock()
	defer registryMu.Unlock()
	codeHosts[h.Provider()] = h
}

func RegisterIssueTracker(t IssueTracker) {
	registryMu.Lock()
	defer registryMu.Unlock()
	issueTrackers[t.Provider()] = t
}

// OnIntegrationsChanged registers a callback for integration rows being
// created, updated or deleted, for providers that hold long-lived
// connections per integration. It runs after the change committed.
func OnIntegrationsChanged(fn func()) {
	registryMu.Lock()
	defer registryMu.Unlock()
	integrationObservers = append(integrationObservers, fn)
}

// IntegrationsChanged tells every observer to reconcile.
func IntegrationsChanged() {
	registryMu.RLock()
	observers := append([]func(){}, integrationObservers...)
	registryMu.RUnlock()
	for _, fn := range observers {
		fn()
	}
}

func RegisterExecutor(e Executor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	executors[e.Name()] = e
}

func ContextProviderFor(kind string) (ContextProvider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := contextProviders[kind]
	return p, ok
}

func TriggerFor(provider string) (TriggerSource, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := triggers[provider]
	return t, ok
}

func ChannelFor(provider string) (Channel, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	c, ok := channels[provider]
	return c, ok
}

func CodeHostFor(provider string) (CodeHost, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	h, ok := codeHosts[provider]
	return h, ok
}

func IssueTrackerFor(provider string) (IssueTracker, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := issueTrackers[provider]
	return t, ok
}

func Executors() []Executor {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Executor, 0, len(executors))
	for _, e := range executors {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// ProviderInfo is what the settings page renders a provider from: the kinds
// it fulfils, its fields, and its setup flow when it has one.
type ProviderInfo struct {
	Provider  string     `json:"provider"`
	Kinds     []string   `json:"kinds"`
	Fields    []Field    `json:"fields"`
	SetupFlow *SetupFlow `json:"setupFlow,omitempty"`
}

// Providers lists every registered provider once, whichever ports it
// implements, sorted by name.
func Providers() []ProviderInfo {
	registryMu.RLock()
	defer registryMu.RUnlock()
	byName := map[string]Provider{}
	for _, t := range triggers {
		byName[t.Provider()] = t
	}
	for _, c := range channels {
		byName[c.Provider()] = c
	}
	for _, h := range codeHosts {
		byName[h.Provider()] = h
	}
	for _, t := range issueTrackers {
		byName[t.Provider()] = t
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		p := byName[name]
		fields := p.Fields()
		if fields == nil {
			fields = []Field{}
		}
		out = append(out, ProviderInfo{Provider: name, Kinds: p.Kinds(), Fields: fields, SetupFlow: p.SetupFlow()})
	}
	return out
}

// ProviderFor resolves a provider by name across every port; the settings
// page validates an integration's config through it.
func ProviderFor(name string) (Provider, bool) {
	for _, info := range Providers() {
		if info.Provider != name {
			continue
		}
		registryMu.RLock()
		defer registryMu.RUnlock()
		if t, ok := triggers[name]; ok {
			return t, true
		}
		if c, ok := channels[name]; ok {
			return c, true
		}
		if h, ok := codeHosts[name]; ok {
			return h, true
		}
		if t, ok := issueTrackers[name]; ok {
			return t, true
		}
	}
	return nil, false
}

// SecretFields names the fields of a provider that hold credentials, the
// ones encrypted at rest and masked on read.
func SecretFields(p Provider) []string {
	var fields []string
	for _, f := range p.Fields() {
		if f.Kind == FieldSecret {
			fields = append(fields, f.Key)
		}
	}
	return fields
}
