package scraper

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// ErrScraperNotFound is returned by Get when no factory is registered for the
// requested source code.
var ErrScraperNotFound = errors.New("scraper not found")

// Factory is a function that constructs a ScraperInterface from a SourceConfig.
// Factories are registered at init() time by each scraper implementation package.
//
// A factory must:
//   - Be safe to call multiple times (idempotent construction)
//   - Return a non-nil ScraperInterface on success
//   - Return a descriptive error if the config is insufficient to build a scraper
type Factory func(cfg sources.SourceConfig) (ScraperInterface, error)

// registry is the package-level singleton that stores all registered factories.
var registry = &scraperRegistry{
	factories: make(map[string]Factory),
}

// scraperRegistry is a thread-safe map from source code → Factory.
type scraperRegistry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// Register adds a factory under name in the global registry.
//
// Register is intended to be called from package init() functions:
//
//	func init() {
//	    scraper.Register("truyenqq", func(cfg sources.SourceConfig) (ScraperInterface, error) {
//	        return truyenqq.New(cfg), nil
//	    })
//	}
//
// Panics if name is empty or if a factory for name is already registered.
// The panic-on-duplicate design is deliberate: a duplicate registration is a
// programming error (two init() functions claiming the same code), not a
// runtime condition that callers should handle.
func Register(name string, factory Factory) {
	if name == "" {
		panic("scraper.Register: name must not be empty")
	}
	if factory == nil {
		panic(fmt.Sprintf("scraper.Register: factory for %q must not be nil", name))
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.factories[name]; exists {
		panic(fmt.Sprintf("scraper.Register: scraper %q is already registered", name))
	}
	registry.factories[name] = factory
}

// Get looks up the factory registered under name, constructs a ScraperInterface
// from cfg, and returns it.
//
// Returns [ErrScraperNotFound] (unwrappable with errors.Is) when no factory is
// registered for name. All other errors come from the factory itself.
func Get(name string, cfg sources.SourceConfig) (ScraperInterface, error) {
	registry.mu.RLock()
	factory, ok := registry.factories[name]
	registry.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("scraper %q: %w", name, ErrScraperNotFound)
	}
	s, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("constructing scraper %q: %w", name, err)
	}
	return s, nil
}

// MustGet is like Get but panics instead of returning an error.
// Use it only in tests or in init() paths where a missing scraper is
// unrecoverable.
func MustGet(name string, cfg sources.SourceConfig) ScraperInterface {
	s, err := Get(name, cfg)
	if err != nil {
		panic(fmt.Sprintf("scraper.MustGet(%q): %v", name, err))
	}
	return s
}

// Names returns all registered scraper names in sorted order.
// The returned slice is a copy; mutating it has no effect on the registry.
func Names() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.factories))
	for name := range registry.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsRegistered reports whether a scraper with the given name has been registered.
func IsRegistered(name string) bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	_, ok := registry.factories[name]
	return ok
}

// unregisterForTest removes a named factory from the global registry.
// It is unexported and intended only for use in tests that need to isolate
// registry state between test cases.
func unregisterForTest(name string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.factories, name)
}
