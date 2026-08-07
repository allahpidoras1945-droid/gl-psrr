package scraper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/example/glukoza/internal/domain"
	"github.com/gocolly/colly/v2"
)

type ScraperConfig struct {
	UserAgent      string
	UserAgents     []string
	Timeout        time.Duration
	MaxDepth       int
	AllowedDomains []string
	Concurrency    int
}

type CollyScraper struct {
	config    ScraperConfig
	extractor domain.Extractor
	random    *rand.Rand
	randomMu  sync.Mutex
}

func NewCollyScraper(cfg ScraperConfig, extractor domain.Extractor) domain.Scraper {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 1
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if extractor == nil {
		extractor = NewRegexExtractor()
	}
	return &CollyScraper{config: cfg, extractor: extractor, random: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func NewHTTP(timeout time.Duration, userAgent string) *CollyScraper {
	return NewCollyScraper(ScraperConfig{Timeout: timeout, UserAgent: userAgent, Concurrency: 1}, NewRegexExtractor()).(*CollyScraper)
}

func (s *CollyScraper) Scrape(ctx context.Context, targetURL string) (*domain.Lead, error) {
	collectorOpts := []colly.CollectorOption{colly.MaxDepth(s.config.MaxDepth), colly.Async(false)}
	if len(s.config.AllowedDomains) > 0 {
		collectorOpts = append(collectorOpts, colly.AllowedDomains(s.config.AllowedDomains...))
	}
	if agent := s.userAgent(); agent != "" {
		collectorOpts = append(collectorOpts, colly.UserAgent(agent))
	}
	c := colly.NewCollector(collectorOpts...)
	c.Context = ctx
	c.SetRequestTimeout(s.config.Timeout)
	c.WithTransport(&http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: s.config.Timeout, DisableKeepAlives: true})
	if err := c.Limit(&colly.LimitRule{DomainRegexp: ".*", Parallelism: s.config.Concurrency}); err != nil {
		return nil, fmt.Errorf("configure scraper limits: %w", err)
	}
	var html strings.Builder
	var companyName string
	var callbackErr error
	var callbackMu sync.Mutex
	c.OnHTML("title", func(element *colly.HTMLElement) {
		if companyName == "" {
			companyName = strings.TrimSpace(element.Text)
		}
	})
	c.OnResponse(func(response *colly.Response) {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body)))
		if err != nil {
			callbackMu.Lock()
			callbackErr = fmt.Errorf("parse %s: %w", targetURL, err)
			callbackMu.Unlock()
			return
		}
		if text, err := document.Html(); err == nil {
			html.WriteString(text)
		} else {
			callbackMu.Lock()
			callbackErr = fmt.Errorf("serialize %s: %w", targetURL, err)
			callbackMu.Unlock()
		}
	})
	c.OnError(func(response *colly.Response, err error) {
		callbackMu.Lock()
		callbackErr = fmt.Errorf("scrape %s (status %d): %w", targetURL, response.StatusCode, err)
		callbackMu.Unlock()
	})
	requestCtx := colly.NewContext()
	requestCtx.Put("target", targetURL)
	if err := c.Request(http.MethodGet, targetURL, nil, requestCtx, nil); err != nil {
		return nil, fmt.Errorf("request %s: %w", targetURL, err)
	}
	callbackMu.Lock()
	err := callbackErr
	callbackMu.Unlock()
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	contacts := s.extractor.ExtractContacts(html.String())
	return &domain.Lead{ID: idFor(targetURL), SourceURL: targetURL, CompanyName: companyName, RawName: companyName, Contacts: contacts, CreatedAt: time.Now().UTC()}, nil
}

func (s *CollyScraper) userAgent() string {
	agents := s.config.UserAgents
	if len(agents) == 0 && s.config.UserAgent != "" {
		return s.config.UserAgent
	}
	if len(agents) == 0 {
		return ""
	}
	s.randomMu.Lock()
	index := s.random.Intn(len(agents))
	s.randomMu.Unlock()
	return agents[index]
}
func idFor(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
