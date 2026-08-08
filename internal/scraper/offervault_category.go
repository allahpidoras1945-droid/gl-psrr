package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/example/glukoza/internal/storage"
	"github.com/gocolly/colly/v2"
)

// offerVaultNetworksURL is a var so tests can point discovery at a local server.
var offerVaultNetworksURL = "https://www.offervault.com/networks/"

// OfferVaultCategoryCrawler discovers OfferVault network profile URLs.
type OfferVaultCategoryCrawler struct {
	config ScraperConfig
	store  *storage.DB
}

// NewOfferVaultCategoryCrawler builds a crawler that skips network URLs already
// persisted in SQLite when store is non-nil.
func NewOfferVaultCategoryCrawler(config ScraperConfig, store *storage.DB) *OfferVaultCategoryCrawler {
	return &OfferVaultCategoryCrawler{config: config, store: store}
}

func (c *OfferVaultCategoryCrawler) DiscoverOfferVaultNetworks(ctx context.Context) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("category crawler context is nil")
	}
	existing := map[string]struct{}{}
	if c.store != nil {
		found, err := c.store.ExistingSourceURLs(ctx)
		if err != nil {
			return nil, err
		}
		existing = found
	}
	base, err := url.Parse(offerVaultNetworksURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OfferVault networks URL: %w", err)
	}
	discovered := make(map[string]struct{})
	ordered := make([]string, 0)
	for pageNumber := 1; ; pageNumber++ {
		pageURL := pageURLFor(base, pageNumber)
		links, err := c.scrapeNetworksPage(ctx, pageURL)
		if err != nil {
			return nil, err
		}
		newOnPage := 0
		for _, link := range links {
			if _, exists := discovered[link]; exists {
				continue
			}
			discovered[link] = struct{}{}
			newOnPage++
			if _, alreadySaved := existing[link]; alreadySaved {
				continue
			}
			ordered = append(ordered, link)
		}
		if len(links) == 0 || newOnPage == 0 {
			break
		}
	}
	return ordered, nil
}

func (c *OfferVaultCategoryCrawler) scrapeNetworksPage(ctx context.Context, pageURL string) ([]string, error) {
	options := []colly.CollectorOption{colly.MaxDepth(1)}
	if c.config.UserAgent != "" {
		options = append(options, colly.UserAgent(c.config.UserAgent))
	}
	collector := colly.NewCollector(options...)
	collector.Context = ctx
	timeout := c.config.Timeout
	if timeout <= 0 {
		timeout = defaultScraperTimeout
	}
	collector.SetRequestTimeout(timeout)
	collector.WithTransport(&http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: timeout, DisableKeepAlives: true})
	links := make([]string, 0)
	var scrapeErr error
	collector.OnResponse(func(response *colly.Response) {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body)))
		if err != nil {
			scrapeErr = fmt.Errorf("parse OfferVault networks page %s: %w", pageURL, err)
			return
		}
		document.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
			href, ok := selection.Attr("href")
			if !ok {
				return
			}
			if networkURL, ok := cleanOfferVaultNetworkURL(href, pageURL); ok {
				links = appendUnique(links, networkURL)
			}
		})
	})
	collector.OnError(func(response *colly.Response, err error) {
		scrapeErr = fmt.Errorf("OfferVault networks request failed for %s [status %d]: %w", pageURL, response.StatusCode, err)
	})
	if err := collector.Request(http.MethodGet, pageURL, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("request OfferVault networks page %s: %w", pageURL, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}
	return links, nil
}

// cleanOfferVaultNetworkURL keeps only /network/<slug> profile links on the same host.
func cleanOfferVaultNetworkURL(rawHref, baseURL string) (string, bool) {
	href := strings.TrimSpace(rawHref)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(strings.ToLower(href), "mailto:") {
		return "", false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", false
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	resolved := base.ResolveReference(parsed)
	if !strings.EqualFold(resolved.Hostname(), base.Hostname()) {
		return "", false
	}
	segments := strings.Split(strings.Trim(resolved.Path, "/"), "/")
	if len(segments) < 2 || !strings.EqualFold(segments[0], "network") || segments[1] == "" {
		return "", false
	}
	resolved.RawQuery, resolved.Fragment = "", ""
	return resolved.String(), true
}
