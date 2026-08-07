package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/example/glukoza/internal/domain"
	"golang.org/x/sync/singleflight"
)

type Manager struct {
	scraper     domain.Scraper
	filter      domain.Filter
	dedup       interface{ IsDuplicate(string) bool }
	tgValidator domain.TGValidator
	workerCount int
	tgMu        sync.RWMutex
	tgCache     map[string]domain.TGValidationResult
	tgFlight    singleflight.Group
}

func NewManager(scraper domain.Scraper, filter domain.Filter, dedup interface{ IsDuplicate(string) bool }, tgValidator domain.TGValidator, workerCount int) domain.Pipeline {
	if workerCount < 1 {
		workerCount = 1
	}
	return &Manager{scraper: scraper, filter: filter, dedup: dedup, tgValidator: tgValidator, workerCount: workerCount, tgCache: make(map[string]domain.TGValidationResult)}
}

func (m *Manager) Run(ctx context.Context, sources []string) ([]*domain.Lead, error) {
	if ctx == nil {
		return nil, fmt.Errorf("pipeline context is nil")
	}
	jobs := make(chan string, len(sources))
	results := make(chan *domain.Lead, m.workerCount)
	jobErrors := make(chan error, len(sources))
	for _, source := range sources {
		jobs <- source
	}
	close(jobs)

	var workers sync.WaitGroup
	for index := 0; index < m.workerCount; index++ {
		workers.Add(1)
		go func(workerID int) {
			defer workers.Done()
			for targetURL := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				lead, err := m.processTarget(ctx, targetURL)
				if err != nil {
					wrapped := fmt.Errorf("worker %d: %w", workerID, err)
					log.Printf("pipeline target failed: %v", wrapped)
					jobErrors <- wrapped
					continue
				}
				if lead == nil {
					continue
				}
				select {
				case results <- lead:
				case <-ctx.Done():
					return
				}
			}
		}(index + 1)
	}

	go func() {
		workers.Wait()
		close(results)
		close(jobErrors)
	}()
	leads := make([]*domain.Lead, 0, len(sources))
	for lead := range results {
		leads = append(leads, lead)
	}
	var runErrors []error
	for err := range jobErrors {
		runErrors = append(runErrors, err)
	}
	if err := ctx.Err(); err != nil {
		return leads, err
	}
	return leads, errors.Join(runErrors...)
}

func (m *Manager) processTarget(ctx context.Context, targetURL string) (*domain.Lead, error) {
	if m.scraper == nil || m.filter == nil {
		return nil, fmt.Errorf("pipeline scraper and filter are required")
	}
	if m.dedup != nil && m.dedup.IsDuplicate("url:"+strings.ToLower(strings.TrimSpace(targetURL))) {
		return nil, nil
	}
	lead, err := m.scraper.Scrape(ctx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("scraping %s: %w", targetURL, err)
	}
	if lead == nil {
		return nil, fmt.Errorf("scraper returned nil lead for %s", targetURL)
	}
	lead.IsCIS, lead.CISReason = m.filter.IsCIS(lead)
	for _, email := range lead.Contacts.Emails {
		if m.dedup != nil {
			m.dedup.IsDuplicate("email:" + email)
		}
	}
	if m.tgValidator == nil {
		return lead, nil
	}
	validHandles := make([]string, 0, len(lead.Contacts.Telegram))
	for _, handle := range lead.Contacts.Telegram {
		result, validateErr := m.validateTelegram(ctx, handle)
		if validateErr != nil {
			return nil, fmt.Errorf("validate Telegram %q: %w", handle, validateErr)
		}
		if result != nil {
			lead.TGResults = append(lead.TGResults, *result)
			if result.Status == domain.TGStatusValid || result.Status == domain.TGStatusSkipped {
				validHandles = append(validHandles, handle)
			}
		}
	}
	lead.Contacts.Telegram = validHandles
	return lead, nil
}

func (m *Manager) validateTelegram(ctx context.Context, handle string) (*domain.TGValidationResult, error) {
	key := strings.ToLower(strings.TrimSpace(handle))
	m.tgMu.RLock()
	cached, ok := m.tgCache[key]
	m.tgMu.RUnlock()
	if ok {
		result := cached
		return &result, nil
	}
	value, err, _ := m.tgFlight.Do(key, func() (interface{}, error) {
		result, validateErr := m.tgValidator.ValidateUsername(ctx, handle)
		if validateErr != nil || result == nil {
			return result, validateErr
		}
		m.tgMu.Lock()
		m.tgCache[key] = *result
		m.tgMu.Unlock()
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	result, ok := value.(*domain.TGValidationResult)
	if !ok {
		return nil, fmt.Errorf("unexpected Telegram validation result type")
	}
	return result, nil
}
