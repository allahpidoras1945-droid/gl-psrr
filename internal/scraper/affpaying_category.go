package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// CategoryCrawler discovers network card pages from Affpaying-style category pages.
type CategoryCrawler struct {
	config ScraperConfig
}

func NewCategoryCrawler(config ScraperConfig) *CategoryCrawler {
	return &CategoryCrawler{config: config}
}

func (c *CategoryCrawler) DiscoverCardURLs(ctx context.Context, categoryURLs []string) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("category crawler context is nil")
	}
	discovered := make(map[string]struct{})
	ordered := make([]string, 0)
	for _, rawCategoryURL := range categoryURLs {
		categoryURL := strings.TrimSpace(rawCategoryURL)
		if categoryURL == "" {
			continue
		}
		parsed, err := url.Parse(categoryURL)
		if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
			return nil, fmt.Errorf("invalid category URL %q", categoryURL)
		}
		categorySeen := make(map[string]struct{})
		for pageNumber := 1; ; pageNumber++ {
			pageURL := pageURLFor(parsed, pageNumber)
			links, err := c.scrapeCategoryPage(ctx, pageURL)
			if err != nil {
				return nil, err
			}
			newOnPage := 0
			for _, link := range links {
				if _, exists := categorySeen[link]; exists {
					continue
				}
				categorySeen[link] = struct{}{}
				newOnPage++
				if _, exists := discovered[link]; exists {
					continue
				}
				discovered[link] = struct{}{}
				ordered = append(ordered, link)
			}
			if len(links) == 0 || newOnPage == 0 {
				break
			}
		}
	}
	return ordered, nil
}

func (c *CategoryCrawler) scrapeCategoryPage(ctx context.Context, pageURL string) ([]string, error) {
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
			scrapeErr = fmt.Errorf("parse category page %s: %w", pageURL, err)
			return
		}
		document.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
			href, ok := selection.Attr("href")
			if !ok {
				return
			}
			if cardURL, ok := cleanCardURL(href, pageURL); ok {
				links = appendUnique(links, cardURL)
			}
		})
	})
	collector.OnError(func(response *colly.Response, err error) {
		scrapeErr = fmt.Errorf("category request failed for %s [status %d]: %w", pageURL, response.StatusCode, err)
	})
	if err := collector.Request(http.MethodGet, pageURL, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("request category page %s: %w", pageURL, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}
	return links, nil
}

const defaultScraperTimeout = 15 * time.Second

var categoryPathBlacklist = map[string]struct{}{
	"affiliate-networks": {}, "affiliate-programs": {}, "categories": {}, "category": {}, "login": {}, "signup": {}, "register": {}, "contact": {}, "about": {}, "search": {}, "page": {},
}

func cleanCardURL(rawHref, baseURL string) (string, bool) {
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
	if len(segments) != 1 || segments[0] == "" {
		return "", false
	}
	segment := strings.ToLower(segments[0])
	if _, blocked := categoryPathBlacklist[segment]; blocked || strings.HasPrefix(segment, "page-") {
		return "", false
	}
	if strings.Contains(segment, "affpaying") || strings.Contains(segment, "affiliatepaying") {
		return "", false
	}
	resolved.RawQuery, resolved.Fragment = "", ""
	return resolved.String(), true
}

func pageURLFor(category *url.URL, pageNumber int) string {
	page := *category
	query := page.Query()
	query.Set("page", fmt.Sprintf("%d", pageNumber))
	page.RawQuery = query.Encode()
	page.Fragment = ""
	return page.String()
}
