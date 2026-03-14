package scraper_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// noopFactory returns a factory that builds a no-op scraper.
func noopFactory() scraper.Factory {
	return func(cfg sources.SourceConfig) (scraper.ScraperInterface, error) {
		return &noopScraper{}, nil
	}
}

// errorFactory returns a factory that always fails with the given error.
func errorFactory(err error) scraper.Factory {
	return func(_ sources.SourceConfig) (scraper.ScraperInterface, error) {
		return nil, err
	}
}

// noopScraper is a minimal ScraperInterface implementation for tests.
type noopScraper struct{}

func (n *noopScraper) GetChapterList(_ context.Context, _ string) ([]sources.Chapter, error) {
	return nil, nil
}
func (n *noopScraper) GetImageURLs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// withRegistered is a test helper that registers name in the global registry,
// runs f, then removes the registration so subsequent tests are not affected.
func withRegistered(t *testing.T, name string, factory scraper.Factory, f func()) {
	t.Helper()
	scraper.Register(name, factory)
	defer scraper.UnregisterForTest(name)
	f()
}

// defaultCfg returns a minimal SourceConfig used in registry tests.
func defaultCfg(code string) sources.SourceConfig {
	return sources.SourceConfig{
		Name:    "Test Source",
		Code:    code,
		BaseURL: "https://example.com",
		Scraper: code,
	}
}

// ── Register ─────────────────────────────────────────────────────────────────

func TestRegister_AddsToRegistry(t *testing.T) {
	withRegistered(t, "test_register", noopFactory(), func() {
		assert.True(t, scraper.IsRegistered("test_register"))
	})
}

func TestRegister_NotRegistered_BeforeCall(t *testing.T) {
	assert.False(t, scraper.IsRegistered("never_registered_xyz"))
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	scraper.Register("test_dup", noopFactory())
	defer scraper.UnregisterForTest("test_dup")

	assert.Panics(t, func() {
		scraper.Register("test_dup", noopFactory())
	}, "registering the same name twice should panic")
}

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	assert.Panics(t, func() {
		scraper.Register("", noopFactory())
	})
}

func TestRegister_PanicsOnNilFactory(t *testing.T) {
	assert.Panics(t, func() {
		scraper.Register("nil_factory_test", nil)
	})
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestGet_ReturnsScraperForRegisteredName(t *testing.T) {
	withRegistered(t, "test_get", noopFactory(), func() {
		s, err := scraper.Get("test_get", defaultCfg("test_get"))
		require.NoError(t, err)
		assert.NotNil(t, s)
	})
}

func TestGet_ReturnsErrScraperNotFound_ForUnknownName(t *testing.T) {
	_, err := scraper.Get("does_not_exist_xyz", defaultCfg("does_not_exist_xyz"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, scraper.ErrScraperNotFound),
		"error should wrap ErrScraperNotFound, got: %v", err)
}

func TestGet_ErrorFromFactory_IsWrapped(t *testing.T) {
	factoryErr := errors.New("missing API key")
	withRegistered(t, "test_factory_err", errorFactory(factoryErr), func() {
		_, err := scraper.Get("test_factory_err", defaultCfg("test_factory_err"))
		require.Error(t, err)
		assert.ErrorIs(t, err, factoryErr,
			"factory error should be wrapped in the returned error")
		assert.NotErrorIs(t, err, scraper.ErrScraperNotFound,
			"factory error should not look like a not-found error")
	})
}

func TestGet_PassesConfigToFactory(t *testing.T) {
	var gotCfg sources.SourceConfig
	factory := func(cfg sources.SourceConfig) (scraper.ScraperInterface, error) {
		gotCfg = cfg
		return &noopScraper{}, nil
	}

	withRegistered(t, "test_cfg_pass", factory, func() {
		cfg := defaultCfg("test_cfg_pass")
		cfg.BaseURL = "https://special-url.com"
		cfg.RateLimitMs = 750

		_, err := scraper.Get("test_cfg_pass", cfg)
		require.NoError(t, err)
		assert.Equal(t, "https://special-url.com", gotCfg.BaseURL)
		assert.Equal(t, 750, gotCfg.RateLimitMs)
	})
}

// ── MustGet ──────────────────────────────────────────────────────────────────

func TestMustGet_ReturnsScraperOnSuccess(t *testing.T) {
	withRegistered(t, "test_mustget", noopFactory(), func() {
		assert.NotPanics(t, func() {
			s := scraper.MustGet("test_mustget", defaultCfg("test_mustget"))
			assert.NotNil(t, s)
		})
	})
}

func TestMustGet_PanicsOnUnknownName(t *testing.T) {
	assert.Panics(t, func() {
		scraper.MustGet("nonexistent_mustget_xyz", defaultCfg("nonexistent_mustget_xyz"))
	})
}

// ── Names ────────────────────────────────────────────────────────────────────

func TestNames_ReturnsSortedNames(t *testing.T) {
	// Register in reverse alphabetical order; expect sorted output.
	for _, name := range []string{"zzz_src", "aaa_src", "mmm_src"} {
		scraper.Register(name, noopFactory())
		defer scraper.UnregisterForTest(name)
	}

	names := scraper.Names()

	// Find our three names in the slice and verify ordering.
	indexOf := func(s string) int {
		for i, n := range names {
			if n == s {
				return i
			}
		}
		return -1
	}

	iA := indexOf("aaa_src")
	iM := indexOf("mmm_src")
	iZ := indexOf("zzz_src")
	require.NotEqual(t, -1, iA, "aaa_src not found in Names()")
	require.NotEqual(t, -1, iM, "mmm_src not found in Names()")
	require.NotEqual(t, -1, iZ, "zzz_src not found in Names()")
	assert.Less(t, iA, iM, "aaa_src should appear before mmm_src")
	assert.Less(t, iM, iZ, "mmm_src should appear before zzz_src")
}

func TestNames_ReturnsCopy(t *testing.T) {
	names1 := scraper.Names()
	names2 := scraper.Names()
	// Mutating the first slice must not affect the second.
	if len(names1) > 0 {
		names1[0] = "mutated"
		assert.NotEqual(t, "mutated", names2[0],
			"Names() should return an independent copy")
	}
}

// ── Concurrency ──────────────────────────────────────────────────────────────

func TestRegistry_ConcurrentReads(t *testing.T) {
	withRegistered(t, "test_concurrent", noopFactory(), func() {
		cfg := defaultCfg("test_concurrent")
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, err := scraper.Get("test_concurrent", cfg)
				assert.NoError(t, err)
				assert.NotNil(t, s)
			}()
		}
		wg.Wait()
	})
}

func TestRegistry_ConcurrentNames(t *testing.T) {
	// Concurrent calls to Names() must not race with each other.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = scraper.Names()
		}()
	}
	wg.Wait()
}
