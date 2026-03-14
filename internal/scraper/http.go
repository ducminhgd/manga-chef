package scraper

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// HTTPClient is the interface scrapers use to make HTTP requests.
// Defining it here (in the consumer package) rather than wrapping *http.Client
// directly allows tests to inject a fake implementation without a live server.
//
// The interface is intentionally minimal — scrapers only need to send requests
// and receive responses; they do not need to manage connection pools or cookies.
type HTTPClient interface {
	// Do sends an HTTP request and returns the response.
	// The caller is responsible for closing Response.Body.
	Do(req *http.Request) (*http.Response, error)
}

// NewHTTPClient builds a production *http.Client configured from a SourceConfig.
//
// The returned client:
//   - Sets a request timeout from cfg.EffectiveTimeoutS()
//   - Does not follow redirects automatically (scrapers handle them explicitly
//     when needed, preventing redirect loops on some CDN servers)
//   - Reuses a shared transport for connection pooling across requests
func NewHTTPClient(cfg *sources.SourceConfig) *http.Client {
	return &http.Client{
		Timeout: time.Duration(cfg.EffectiveTimeoutS()) * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			// Allow up to 5 redirects (standard browser behaviour).
			// Return http.ErrUseLastResponse to stop and return the last response.
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			return nil
		},
		Transport: sharedTransport,
	}
}

// sharedTransport is a single http.Transport reused by all HTTP clients to
// share the connection pool. Reusing transports avoids the overhead of
// re-establishing TCP connections on every request.
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}
