package access

import (
	"fmt"
	"sort"
	"sync"
)

// Config is a source as the profile stores it: a name, the provider to open
// it with, the domains it may answer (empty means every domain the provider
// implements) and the provider's own settings.
type Config struct {
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Domains  []Domain          `json:"domains,omitempty"`
	Settings map[string]string `json:"config,omitempty"`
}

// Factory opens a source from its configuration.
type Factory func(cfg Config) (Source, error)

var (
	registryMu sync.RWMutex
	factories  = map[string]Factory{}
)

// Register makes a provider available to Open. Adapters call it from init.
func Register(provider string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	factories[provider] = factory
}

// Providers lists the registered provider keys, sorted.
func Providers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Open builds a source from its configuration through the registered
// factory, then checks the configured domains against what it implements.
func Open(cfg Config) (Source, error) {
	registryMu.RLock()
	factory, ok := factories[cfg.Provider]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w %q (registered: %v)", ErrUnknownProvider, cfg.Provider, Providers())
	}
	source, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", cfg.Name, err)
	}
	for _, domain := range cfg.Domains {
		if !Implements(source, domain) {
			return nil, fmt.Errorf("source %s: provider %s does not answer %s (it answers %v)", cfg.Name, cfg.Provider, domain, ImplementedDomains(source))
		}
	}
	return source, nil
}
