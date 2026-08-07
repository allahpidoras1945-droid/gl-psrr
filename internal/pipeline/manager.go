package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

type executionStats struct {
	targets     atomic.Uint64
	leads       atomic.Uint64
	cis         atomic.Uint64
	nonCIS      atomic.Uint64
	tgExtracted atomic.Uint64
	tgChecked   atomic.Uint64
	tgValid     atomic.Uint64
	tgInvalid   atomic.Uint64
	tgNotFound  atomic.Uint64
	tgSkipped   atomic.Uint64
	tgDeleted   atomic.Uint64
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
	started := time.Now()
	stats := &executionStats{}
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
				stats.targets.Add(1)
				select {
				case <-ctx.Done():
					return
				default:
				}
				lead, err := m.processTarget(ctx, targetURL, stats)
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
	printSummary(stats, time.Since(started))
	if err := ctx.Err(); err != nil {
		return leads, err
	}
	return leads, errors.Join(runErrors...)
}

func (m *Manager) processTarget(ctx context.Context, targetURL string, stats *executionStats) (*domain.Lead, error) {
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
	stats.leads.Add(1)
	lead.IsCIS, lead.CISReason = m.filter.IsCIS(lead)
	if lead.IsCIS {
		stats.cis.Add(1)
	} else {
		stats.nonCIS.Add(1)
	}
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
		stats.tgExtracted.Add(1)
		result, validateErr := m.validateTelegram(ctx, handle)
		if validateErr != nil {
			return nil, fmt.Errorf("validate Telegram %q: %w", handle, validateErr)
		}
		if result != nil {
			stats.tgChecked.Add(1)
			stats.recordTelegramStatus(result.Status)
			lead.TGResults = append(lead.TGResults, *result)
			if result.Status == domain.TGStatusValid || result.Status == domain.TGStatusSkipped {
				validHandles = append(validHandles, handle)
			}
		}
	}
	lead.Contacts.Telegram = validHandles
	return lead, nil
}

func (s *executionStats) recordTelegramStatus(status domain.TGStatus) {
	switch status {
	case domain.TGStatusValid:
		s.tgValid.Add(1)
	case domain.TGStatusInvalid:
		s.tgInvalid.Add(1)
	case domain.TGStatusNotFound:
		s.tgNotFound.Add(1)
	case domain.TGStatusSkipped:
		s.tgSkipped.Add(1)
	case domain.TGStatusDeleted:
		s.tgDeleted.Add(1)
	}
}

func printSummary(stats *executionStats, elapsed time.Duration) {
	targets := stats.targets.Load()
	cis := stats.cis.Load()
	nonCIS := stats.nonCIS.Load()
	totalClassified := cis + nonCIS
	percent := func(value, total uint64) float64 {
		if total == 0 {
			return 0
		}
		return float64(value) * 100 / float64(total)
	}
	speed := 0.0
	if elapsed > 0 {
		speed = float64(targets) / elapsed.Seconds()
	}
	fmt.Println("====================================================")
	fmt.Println("                EXECUTION SUMMARY")
	fmt.Println("====================================================")
	fmt.Printf(" Total URLs Processed : %d\n", targets)
	fmt.Printf(" Total Leads Extracted: %d\n", stats.leads.Load())
	fmt.Printf(" CIS Filtered (Flagged): %d (%.1f%%)\n", cis, percent(cis, totalClassified))
	fmt.Printf(" Non-CIS Leads        : %d (%.1f%%)\n", nonCIS, percent(nonCIS, totalClassified))
	fmt.Printf(" Telegram Handles     : %d extracted, %d checked\n", stats.tgExtracted.Load(), stats.tgChecked.Load())
	fmt.Printf("   - Valid TG Accounts : %d\n", stats.tgValid.Load())
	fmt.Printf("   - Not Found         : %d\n", stats.tgNotFound.Load())
	fmt.Printf("   - Invalid           : %d\n", stats.tgInvalid.Load())
	fmt.Printf("   - Skipped           : %d\n", stats.tgSkipped.Load())
	fmt.Printf("   - Deleted           : %d\n", stats.tgDeleted.Load())
	fmt.Printf(" Total Elapsed Time   : %.2fs\n", elapsed.Seconds())
	fmt.Printf(" Processing Speed     : ~%.1f targets/sec\n", speed)
	fmt.Println("====================================================")
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
