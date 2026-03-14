// export_test.go exposes internal symbols to the scraper_test package.
// This file is only compiled during `go test`; it is invisible to consumers.
package scraper

// UnregisterForTest removes a factory from the global registry.
// It is only available in tests (this file has no build tag but lives in the
// non-_test package, so it compiles only when the package is under test via
// the `package scraper` declaration in registry_test.go companions).
//
// Use it to clean up after tests that call Register, preventing state leakage
// between test cases:
//
//	scraper.Register("my_scraper", factory)
//	defer scraper.UnregisterForTest("my_scraper")
var UnregisterForTest = unregisterForTest
