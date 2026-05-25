// Crawler types live in the standalone internal/crawl/ package since
// the T04 extraction. This file forwards the symbols so existing
// callers (enrichtopics.go, enrichtopics_test.go, cmd/sea-noise) keep
// compiling without a sweeping rename. New consumers should import
// sea-noise/internal/crawl directly.
package enrichtopics

import "sea-noise/internal/crawl"

// Type aliases preserve identity — these are the exact same types as
// the ones in internal/crawl, just reachable through this package.
type (
	Page        = crawl.Page
	Crawler     = crawl.Crawler
	NoopCrawler = crawl.NoopCrawler
	CFCrawler   = crawl.CFCrawler
)

// ErrCrawl forwards the sentinel from internal/crawl.
var ErrCrawl = crawl.ErrCrawl

// NewCFCrawler forwards to crawl.NewCFCrawler.
func NewCFCrawler(accountID, apiToken string) *CFCrawler {
	return crawl.NewCFCrawler(accountID, apiToken)
}

// NewCrawlerFromEnv forwards to crawl.NewCrawlerFromEnv.
func NewCrawlerFromEnv() Crawler {
	return crawl.NewCrawlerFromEnv()
}
