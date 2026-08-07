package filter

import (
	"sync"
	"testing"

	"github.com/example/glukoza/internal/domain"
)

func TestCISFilterReportsAuditableReasons(t *testing.T) {
	filter := &CIS{}
	cases := []struct {
		name   string
		lead   *domain.Lead
		reason string
	}{
		{"provider", &domain.Lead{Contacts: domain.ContactInfo{Emails: []string{"person@mail.ru"}}}, "Email CIS TLD match: .ru"},
		{"suffix", &domain.Lead{RawName: "Petrenko"}, "Name match: Slavic surname suffix in [petrenko]"},
		{"name", &domain.Lead{RawName: "Dmitry Smith"}, "Name match: Slavic name token [dmitry]"},
		{"telegram", &domain.Lead{Contacts: domain.ContactInfo{Telegram: []string{"anna_sales"}}}, "Telegram handle match: Slavic name token [anna]"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			isCIS, reason := filter.IsCIS(test.lead)
			if !isCIS || reason != test.reason {
				t.Fatalf("got (%v, %q), want (%v, %q)", isCIS, reason, true, test.reason)
			}
		})
	}
}

func TestCISFilterAvoidsSystemAndEnglishEmailPrefixFalsePositives(t *testing.T) {
	filter := &CIS{}
	for _, localPart := range []string{"admin", "info", "support", "contact", "sales", "help", "billing", "justin", "main", "fin", "bin", "win", "domain"} {
		isCIS, reason := filter.IsCIS(&domain.Lead{Contacts: domain.ContactInfo{Emails: []string{localPart + "@example.com"}}})
		if isCIS || reason != "" {
			t.Errorf("%s@example.com classified as CIS: %q", localPart, reason)
		}
	}
}

func TestCISFilterRecognizesAdditionalNames(t *testing.T) {
	filter := &CIS{}
	for _, name := range []string{"viktoriia", "victoria", "vlad", "bogdan", "danylo", "daria"} {
		isCIS, reason := filter.IsCIS(&domain.Lead{RawName: name})
		if !isCIS || reason == "" {
			t.Errorf("%s was not classified as CIS", name)
		}
	}
}

func TestDeduplicatorNormalizesAndIsSafeConcurrently(t *testing.T) {
	deduplicator := NewDeduplicator()
	var group sync.WaitGroup
	firstCount := 0
	var mu sync.Mutex
	for index := 0; index < 200; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if !deduplicator.IsDuplicate("  Lead@Example.COM ") {
				mu.Lock()
				firstCount++
				mu.Unlock()
			}
		}()
	}
	group.Wait()
	if firstCount != 1 {
		t.Fatalf("first-seen count = %d, want 1", firstCount)
	}
	if !deduplicator.IsDuplicate("   ") {
		t.Fatal("empty identifier should be treated as duplicate")
	}
}
