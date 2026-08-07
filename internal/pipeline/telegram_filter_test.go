package pipeline

import (
	"context"
	"testing"

	"github.com/example/glukoza/internal/domain"
)

type statusValidator struct{}

func (statusValidator) ValidateUsername(_ context.Context, username string) (*domain.TGValidationResult, error) {
	status := domain.TGStatusNotFound
	if username == "available" {
		status = domain.TGStatusValid
	}
	return &domain.TGValidationResult{Username: username, Status: status}, nil
}
func (statusValidator) Close() error { return nil }

type contactScraper struct{}

func (contactScraper) Scrape(_ context.Context, targetURL string) (*domain.Lead, error) {
	return &domain.Lead{ID: targetURL, SourceURL: targetURL, Contacts: domain.ContactInfo{Telegram: []string{"missing", "available"}}}, nil
}

func TestManagerRemovesUnavailableTelegramHandles(t *testing.T) {
	manager := NewManager(contactScraper{}, fakeFilter{}, &fakeDedup{}, statusValidator{}, 1)
	leads, err := manager.Run(context.Background(), []string{"https://contacts.example"})
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("lead count = %d", len(leads))
	}
	if len(leads[0].Contacts.Telegram) != 1 || leads[0].Contacts.Telegram[0] != "available" {
		t.Fatalf("telegram contacts = %#v", leads[0].Contacts.Telegram)
	}
	if len(leads[0].TGResults) != 2 {
		t.Fatalf("validation results = %#v", leads[0].TGResults)
	}
}
