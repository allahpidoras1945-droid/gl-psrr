package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestAffpayingScraperExtractsHiddenContactsManagersAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<html><head><title>Fallback Network</title></head><body>
		<h1>Acme CPA Network</h1>
		<table><tr><th>Commission Type</th><td>CPA / CPL</td></tr><tr><th>Payment Frequency</th><td>Net 30</td></tr><tr><th>Minimum Payout</th><td>$100</td></tr><tr><th>Referral Rate</th><td>5%</td></tr>
		<tr><th>Affiliate Managers</th><td><span class="manager-name">Alice Manager</span><a data-email="alice@acme.example" data-tippy="skype:alice_manager @alice_acme" href="https://t.me/alice_acme">contact</a></td></tr></table>
		<a title="support@acme.example" href="mailto:support@acme.example" data-email="billing@acme.example">Support</a>
		</body></html>`))
	}))
	defer server.Close()

	scraper := NewAffpayingScraper(ScraperConfig{Timeout: time.Second, UserAgent: "test-agent"}, NewRegexExtractor())
	lead, err := scraper.Scrape(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if lead.CompanyName != "Acme CPA Network" {
		t.Fatalf("company = %q", lead.CompanyName)
	}
	emails := append([]string(nil), lead.Contacts.Emails...)
	sort.Strings(emails)
	if !reflect.DeepEqual(emails, []string{"alice@acme.example", "billing@acme.example", "support@acme.example"}) {
		t.Fatalf("emails = %#v", lead.Contacts.Emails)
	}
	if !reflect.DeepEqual(lead.Contacts.Telegram, []string{"alice_acme"}) {
		t.Fatalf("telegram = %#v", lead.Contacts.Telegram)
	}
	if lead.Network == nil || lead.Network.CommissionType != "CPA / CPL" || lead.Network.PaymentFrequency != "Net 30" || lead.Network.MinimumPayout != "$100" || lead.Network.ReferralRate != "5%" {
		t.Fatalf("metadata = %#v", lead.Network)
	}
	if len(lead.Network.Managers) != 1 || lead.Network.Managers[0].Name != "Alice Manager" {
		t.Fatalf("managers = %#v", lead.Network.Managers)
	}
}
