package scraper

import (
	"context"
	"testing"
)

func TestScraperFactoryRoutesKnownAndDefaultHosts(t *testing.T) {
	factory, ok := NewScraperFactory(ScraperConfig{}, NewRegexExtractor()).(*Factory)
	if !ok {
		t.Fatal("constructor did not return a Factory")
	}
	for _, host := range []string{"affpaying.com", "www.affpaying.com"} {
		scraper, exists := factory.routes[host]
		if !exists {
			t.Fatalf("missing route for %s", host)
		}
		if _, isAffpaying := scraper.(*AffpayingScraper); !isAffpaying {
			t.Fatalf("route %s uses %T", host, scraper)
		}
	}
	if _, isDefault := factory.defaultScraper.(*CollyScraper); !isDefault {
		t.Fatalf("default scraper uses %T", factory.defaultScraper)
	}
	if _, err := factory.Scrape(context.Background(), "not-a-url"); err == nil {
		t.Fatal("expected invalid URL error")
	}
}
