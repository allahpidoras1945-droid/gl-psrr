package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/example/glukoza/internal/domain"
	"github.com/gocolly/colly/v2"
)

type AffpayingScraper struct {
	config    ScraperConfig
	extractor domain.Extractor
}

func NewAffpayingScraper(cfg ScraperConfig, extractor domain.Extractor) domain.Scraper {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if extractor == nil {
		extractor = NewRegexExtractor()
	}
	return &AffpayingScraper{config: cfg, extractor: extractor}
}

func (s *AffpayingScraper) Scrape(ctx context.Context, targetURL string) (*domain.Lead, error) {
	if ctx == nil {
		return nil, fmt.Errorf("scraper context is nil")
	}
	options := []colly.CollectorOption{colly.MaxDepth(1)}
	if s.config.UserAgent != "" {
		options = append(options, colly.UserAgent(s.config.UserAgent))
	}
	if len(s.config.AllowedDomains) > 0 {
		options = append(options, colly.AllowedDomains(s.config.AllowedDomains...))
	}
	collector := colly.NewCollector(options...)
	collector.Context = ctx
	collector.SetRequestTimeout(s.config.Timeout)
	collector.WithTransport(&http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: s.config.Timeout, DisableKeepAlives: true})

	var pageHTML strings.Builder
	var parsedLead *domain.Lead
	var scrapeErr error
	collector.OnResponse(func(response *colly.Response) {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body)))
		if err != nil {
			scrapeErr = fmt.Errorf("parse CPA page %s: %w", targetURL, err)
			return
		}
		if html, err := document.Html(); err == nil {
			pageHTML.WriteString(html)
		}
		parsedLead = parseAffpayingDocument(document, targetURL)
	})
	collector.OnError(func(response *colly.Response, err error) {
		scrapeErr = fmt.Errorf("CPA scraper request failed for %s [status %d]: %w", targetURL, response.StatusCode, err)
	})
	if err := collector.Request(http.MethodGet, targetURL, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("request CPA page %s: %w", targetURL, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}
	if parsedLead == nil {
		return nil, fmt.Errorf("CPA scraper returned no page for %s", targetURL)
	}
	parsedLead.Contacts = s.extractor.ExtractContacts(pageHTML.String())
	for index := range parsedLead.Network.Managers {
		manager := &parsedLead.Network.Managers[index]
		contacts := s.extractor.ExtractContacts(strings.Join([]string{manager.Name, strings.Join(manager.Emails, " "), strings.Join(manager.Skype, " "), strings.Join(manager.Telegram, " ")}, " "))
		if len(manager.Emails) == 0 {
			manager.Emails = contacts.Emails
		}
		if len(manager.Skype) == 0 {
			manager.Skype = contacts.Skype
		}
		if len(manager.Telegram) == 0 {
			manager.Telegram = contacts.Telegram
		}
	}
	return parsedLead, nil
}

func parseAffpayingDocument(document *goquery.Document, targetURL string) *domain.Lead {
	companyName := strings.TrimSpace(document.Find("h1").First().Text())
	if companyName == "" {
		companyName = strings.TrimSpace(document.Find("title").First().Text())
	}
	metadata := &domain.NetworkMetadata{}
	document.Find("tr").Each(func(_ int, row *goquery.Selection) { parseMetadataRow(row, metadata) })
	document.Find("dt").Each(func(_ int, label *goquery.Selection) { parseMetadataPair(label.Text(), label.Next().Text(), metadata) })
	managers := make([]domain.AffiliateManager, 0)
	document.Find("tr, li, div").Each(func(_ int, block *goquery.Selection) {
		text := strings.TrimSpace(strings.Join(strings.Fields(block.Text()), " "))
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "affiliate manager") && !strings.Contains(lower, "account manager") {
			return
		}
		manager := domain.AffiliateManager{Name: managerName(block, text)}
		block.Find("[data-email], [data-tippy], [title], a[href]").Each(func(_ int, node *goquery.Selection) {
			for _, attribute := range []string{"data-email", "data-tippy", "title", "href"} {
				if value, ok := node.Attr(attribute); ok {
					addManagerContact(&manager, value)
				}
			}
		})
		if manager.Name != "" {
			managers = appendManager(managers, manager)
		}
	})
	metadata.Managers = managers
	return &domain.Lead{ID: idFor(targetURL), SourceURL: targetURL, CompanyName: companyName, RawName: companyName, Network: metadata, CreatedAt: time.Now().UTC()}
}

func parseMetadataRow(row *goquery.Selection, metadata *domain.NetworkMetadata) {
	cells := row.Find("th, td")
	if cells.Length() < 2 {
		return
	}
	parseMetadataPair(cells.Eq(0).Text(), cells.Eq(1).Text(), metadata)
}
func parseMetadataPair(label, value string, metadata *domain.NetworkMetadata) {
	label = strings.ToLower(strings.TrimSpace(strings.Join(strings.Fields(label), " ")))
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "" {
		return
	}
	switch {
	case strings.Contains(label, "commission") || strings.Contains(label, "payout model") || strings.Contains(label, "payment model"):
		metadata.CommissionType = value
	case strings.Contains(label, "frequency") || strings.Contains(label, "payment terms"):
		metadata.PaymentFrequency = value
	case strings.Contains(label, "minimum") && strings.Contains(label, "payout"):
		metadata.MinimumPayout = value
	case strings.Contains(label, "referral"):
		metadata.ReferralRate = value
	}
}
func managerName(block *goquery.Selection, text string) string {
	for _, selector := range []string{".name", ".manager-name", "strong", "b", "h3", "h4"} {
		if value := strings.TrimSpace(block.Find(selector).First().Text()); value != "" {
			return value
		}
	}
	parts := strings.SplitN(text, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0])
	}
	return ""
}
func addManagerContact(manager *domain.AffiliateManager, value string) {
	contacts := (&RegexExtractor{}).ExtractContacts(value)
	manager.Emails = appendUnique(manager.Emails, contacts.Emails...)
	manager.Skype = appendUnique(manager.Skype, contacts.Skype...)
	manager.Telegram = appendUnique(manager.Telegram, contacts.Telegram...)
}
func appendManager(managers []domain.AffiliateManager, candidate domain.AffiliateManager) []domain.AffiliateManager {
	for index := range managers {
		if strings.EqualFold(managers[index].Name, candidate.Name) {
			managers[index].Emails = appendUnique(managers[index].Emails, candidate.Emails...)
			managers[index].Skype = appendUnique(managers[index].Skype, candidate.Skype...)
			managers[index].Telegram = appendUnique(managers[index].Telegram, candidate.Telegram...)
			return managers
		}
	}
	return append(managers, candidate)
}
