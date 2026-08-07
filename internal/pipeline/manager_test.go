package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/example/glukoza/internal/domain"
)

type fakeScraper struct{}

func (fakeScraper) Scrape(_ context.Context, target string) (*domain.Lead, error) {
	if strings.Contains(target, "bad") {
		return nil, errors.New("unreachable")
	}
	return &domain.Lead{ID: target, SourceURL: target, Contacts: domain.ContactInfo{Telegram: []string{"shared_handle"}}}, nil
}

type fakeFilter struct{}

func (fakeFilter) IsCIS(*domain.Lead) (bool, string) { return false, "" }
func (fakeFilter) IsDuplicate(string) bool           { return false }

type fakeDedup struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func (d *fakeDedup) IsDuplicate(value string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = map[string]struct{}{}
	}
	if _, ok := d.seen[value]; ok {
		return true
	}
	d.seen[value] = struct{}{}
	return false
}

type fakeValidator struct {
	mu    sync.Mutex
	calls int
}

func (v *fakeValidator) ValidateUsername(_ context.Context, username string) (*domain.TGValidationResult, error) {
	v.mu.Lock()
	v.calls++
	v.mu.Unlock()
	return &domain.TGValidationResult{Username: username, Status: domain.TGStatusValid}, nil
}
func (*fakeValidator) Close() error { return nil }

func TestManagerContinuesAfterTargetFailureAndDeduplicatesHandles(t *testing.T) {
	validator := &fakeValidator{}
	manager := NewManager(fakeScraper{}, fakeFilter{}, &fakeDedup{}, validator, 4)
	leads, err := manager.Run(context.Background(), []string{"https://good-one", "https://bad", "https://good-two", "https://good-one"})
	if len(leads) != 2 {
		t.Fatalf("lead count = %d, want 2", len(leads))
	}
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("error = %v, want target failure", err)
	}
	validator.mu.Lock()
	calls := validator.calls
	validator.mu.Unlock()
	if calls != 1 {
		t.Fatalf("validator calls = %d, want 1", calls)
	}
}

func TestManagerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := NewManager(fakeScraper{}, fakeFilter{}, &fakeDedup{}, nil, 2)
	_, err := manager.Run(ctx, []string{"https://one"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
