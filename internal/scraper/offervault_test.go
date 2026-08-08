package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/example/glukoza/internal/domain"
	"github.com/example/glukoza/internal/storage"
)

func TestOfferVaultScraperExtractsHiddenContactsManagersAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<html><head><title>Fallback Network</title><meta property="og:title" content="Zenith Network"></head><body>
		<table><tr><th>Commission Type</th><td>CPA / RevShare</td></tr><tr><th>Minimum Payout</th><td>$50</td></tr>
		<tr><th>Network Contact</th><td><span class="manager-name">Bob Manager</span><a data-email="bob@zenith.example" data-tippy="skype:bob_manager @bob_zenith" href="https://t.me/bob_zenith">contact</a></td></tr></table>
		<a title="sales@zenith.example" href="mailto:sales@zenith.example">Sales</a>
		</body></html>`))
	}))
	defer server.Close()

	scraper := NewOfferVaultScraper(ScraperConfig{Timeout: time.Second, UserAgent: "test-agent"}, NewRegexExtractor())
	lead, err := scraper.Scrape(context.Background(), server.URL+"/network/zenith")
	if err != nil {
		t.Fatal(err)
	}
	if lead.CompanyName != "Zenith Network" {
		t.Fatalf("company = %q", lead.CompanyName)
	}
	emails := append([]string(nil), lead.Contacts.Emails...)
	sort.Strings(emails)
	if !reflect.DeepEqual(emails, []string{"bob@zenith.example", "sales@zenith.example"}) {
		t.Fatalf("emails = %#v", lead.Contacts.Emails)
	}
	if !reflect.DeepEqual(lead.Contacts.Telegram, []string{"bob_zenith"}) {
		t.Fatalf("telegram = %#v", lead.Contacts.Telegram)
	}
	if lead.Network == nil || lead.Network.CommissionType != "CPA / RevShare" || lead.Network.MinimumPayout != "$50" {
		t.Fatalf("metadata = %#v", lead.Network)
	}
	if len(lead.Network.Managers) != 1 || lead.Network.Managers[0].Name != "Bob Manager" {
		t.Fatalf("managers = %#v", lead.Network.Managers)
	}
}

func TestOfferVaultCategoryCrawlerFiltersNetworkLinksAndSkipsKnownURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("page") {
		case "1", "":
			_, _ = writer.Write([]byte(`<a href="/network/alpha">Alpha</a><a href="/networks/">Listing</a><a href="https://other.example/network/outside">Outside</a>`))
		case "2":
			_, _ = writer.Write([]byte(`<a href="/network/beta?ref=directory">Beta</a><a href="/network/alpha">Duplicate</a>`))
		default:
			_, _ = writer.Write([]byte(`<html><body>No networks</body></html>`))
		}
	}))
	defer server.Close()

	original := offerVaultNetworksURL
	offerVaultNetworksURL = server.URL + "/networks/"
	defer func() { offerVaultNetworksURL = original }()

	database, err := storage.InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	crawler := NewOfferVaultCategoryCrawler(ScraperConfig{Timeout: time.Second}, database)
	got, err := crawler.DiscoverOfferVaultNetworks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{server.URL + "/network/alpha", server.URL + "/network/beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("networks = %#v, want %#v", got, want)
	}

	// Persist the first discovered network, then confirm a second crawl skips it.
	if err := database.SaveLead(context.Background(), &domain.Lead{ID: "existing", CompanyName: "Alpha Network", SourceURL: got[0], CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	second, err := NewOfferVaultCategoryCrawler(ScraperConfig{Timeout: time.Second}, database).DiscoverOfferVaultNetworks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, []string{got[1]}) {
		t.Fatalf("second discovery = %#v, want only %q", second, got[1])
	}
}
