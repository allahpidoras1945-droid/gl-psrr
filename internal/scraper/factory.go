package scraper

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/example/glukoza/internal/domain"
)

// Factory dispatches each target URL to the scraper specialized for its host.
type Factory struct {
	routes         map[string]domain.Scraper
	defaultScraper domain.Scraper
}

// NewScraperFactory builds the default scraper registry.
func NewScraperFactory(cfg ScraperConfig, extractor domain.Extractor) domain.Scraper {
	defaultScraper := NewCollyScraper(cfg, extractor)
	affpaying := NewAffpayingScraper(cfg, extractor)
	offervault := NewOfferVaultScraper(cfg, extractor)
	return &Factory{
		routes: map[string]domain.Scraper{
			"affpaying.com":      affpaying,
			"www.affpaying.com":  affpaying,
			"offervault.com":     offervault,
			"www.offervault.com": offervault,
		},
		defaultScraper: defaultScraper,
	}
}

// Scrape parses the target URL and delegates to its registered host scraper.
func (f *Factory) Scrape(ctx context.Context, targetURL string) (*domain.Lead, error) {
	if f == nil || f.defaultScraper == nil {
		return nil, fmt.Errorf("scraper factory is not configured")
	}
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return nil, fmt.Errorf("parse target URL %q: %w", targetURL, err)
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || parsed.Scheme == "" {
		return nil, fmt.Errorf("target URL %q must include a scheme and host", targetURL)
	}
	if selected, ok := f.routes[host]; ok {
		return selected.Scrape(ctx, targetURL)
	}
	return f.defaultScraper.Scrape(ctx, targetURL)
}

var _ domain.Scraper = (*Factory)(nil)
