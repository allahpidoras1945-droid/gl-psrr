package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/glukoza/internal/domain"
	_ "modernc.org/sqlite"
)

type DB struct{ sql *sql.DB }

func InitDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	database.SetMaxOpenConns(1)
	manager := &DB{sql: database}
	if _, err := database.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return manager, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS leads (
 id TEXT PRIMARY KEY,
 company_name TEXT NOT NULL,
 source_url TEXT UNIQUE NOT NULL,
 source_domain TEXT NOT NULL,
 emails TEXT,
 telegram_handles TEXT,
 tg_validation_status TEXT,
 skype TEXT,
 linkedin TEXT,
 twitter TEXT,
 is_cis BOOLEAN DEFAULT 0,
 cis_reason TEXT,
 mx_valid BOOLEAN DEFAULT 0,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_leads_source_domain ON leads(source_domain);
CREATE INDEX IF NOT EXISTS idx_leads_is_cis ON leads(is_cis);
`

func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

func (d *DB) SaveLead(ctx context.Context, lead *domain.Lead) error {
	if d == nil || d.sql == nil {
		return fmt.Errorf("database is not initialized")
	}
	if lead == nil {
		return fmt.Errorf("lead is nil")
	}
	domainName := sourceDomain(lead.SourceURL)
	_, err := d.sql.ExecContext(ctx, `
INSERT INTO leads (id, company_name, source_url, source_domain, emails, telegram_handles, tg_validation_status, skype, linkedin, twitter, is_cis, cis_reason, mx_valid)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_url) DO UPDATE SET
 company_name = CASE WHEN excluded.company_name <> '' THEN excluded.company_name ELSE leads.company_name END,
 emails = CASE WHEN excluded.emails <> '' THEN excluded.emails ELSE leads.emails END,
 telegram_handles = CASE WHEN excluded.telegram_handles <> '' THEN excluded.telegram_handles ELSE leads.telegram_handles END,
 tg_validation_status = CASE WHEN excluded.tg_validation_status <> '' THEN excluded.tg_validation_status ELSE leads.tg_validation_status END,
 skype = CASE WHEN excluded.skype <> '' THEN excluded.skype ELSE leads.skype END,
 linkedin = CASE WHEN excluded.linkedin <> '' THEN excluded.linkedin ELSE leads.linkedin END,
 twitter = CASE WHEN excluded.twitter <> '' THEN excluded.twitter ELSE leads.twitter END,
 is_cis = excluded.is_cis,
 cis_reason = CASE WHEN excluded.cis_reason <> '' THEN excluded.cis_reason ELSE leads.cis_reason END,
 mx_valid = excluded.mx_valid,
 updated_at = CURRENT_TIMESTAMP`, lead.ID, lead.CompanyName, lead.SourceURL, domainName, join(lead.Contacts.Emails), join(lead.Contacts.Telegram), tgStatuses(lead.TGResults), join(lead.Contacts.Skype), join(lead.Contacts.LinkedIn), join(lead.Contacts.Twitter), lead.IsCIS, lead.CISReason, lead.MXValid)
	if err != nil {
		return fmt.Errorf("save lead %q: %w", lead.SourceURL, err)
	}
	return nil
}

// ExistingSourceURLs returns every source_url already recorded, used to skip
// re-discovering the same network profiles across crawler runs.
func (d *DB) ExistingSourceURLs(ctx context.Context) (map[string]struct{}, error) {
	if d == nil || d.sql == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := d.sql.QueryContext(ctx, `SELECT source_url FROM leads`)
	if err != nil {
		return nil, fmt.Errorf("query existing source URLs: %w", err)
	}
	defer rows.Close()
	existing := make(map[string]struct{})
	for rows.Next() {
		var sourceURL string
		if err := rows.Scan(&sourceURL); err != nil {
			return nil, fmt.Errorf("scan source URL: %w", err)
		}
		existing[sourceURL] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source URLs: %w", err)
	}
	return existing, nil
}

func (d *DB) GetLeads(ctx context.Context, filterCIS bool) ([]*domain.Lead, error) {
	if d == nil || d.sql == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	query := `SELECT id, company_name, source_url, source_domain, emails, telegram_handles, tg_validation_status, skype, linkedin, twitter, is_cis, cis_reason, mx_valid, created_at FROM leads`
	args := []any{}
	if filterCIS {
		query += ` WHERE is_cis = 1`
	}
	query += ` ORDER BY created_at, id`
	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query leads: %w", err)
	}
	defer rows.Close()
	leads := make([]*domain.Lead, 0)
	for rows.Next() {
		var lead domain.Lead
		var sourceDomain, emails, telegram, statuses, skype, linkedin, twitter, cisReason, created string
		var isCIS, mxValid bool
		if err := rows.Scan(&lead.ID, &lead.CompanyName, &lead.SourceURL, &sourceDomain, &emails, &telegram, &statuses, &skype, &linkedin, &twitter, &isCIS, &cisReason, &mxValid, &created); err != nil {
			return nil, fmt.Errorf("scan lead: %w", err)
		}
		lead.RawName, lead.Contacts = lead.CompanyName, domain.ContactInfo{Emails: split(emails), Telegram: split(telegram), Skype: split(skype), LinkedIn: split(linkedin), Twitter: split(twitter)}
		lead.IsCIS, lead.CISReason, lead.TGResults = isCIS, cisReason, parseStatuses(statuses)
		lead.MXValid = mxValid
		lead.CreatedAt = parseTime(created)
		leads = append(leads, &lead)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leads: %w", err)
	}
	return leads, nil
}

func sourceDomain(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
func join(values []string) string { return strings.Join(values, ",") }
func split(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
func tgStatuses(results []domain.TGValidationResult) string {
	values := make([]string, 0, len(results))
	for _, result := range results {
		values = append(values, fmt.Sprintf("@%s:[%s]", result.Username, result.Status))
	}
	return strings.Join(values, ", ")
}
func parseStatuses(raw string) []domain.TGValidationResult {
	result := make([]domain.TGValidationResult, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		item = strings.TrimPrefix(item, "@")
		parts := strings.SplitN(item, ":[", 2)
		if len(parts) != 2 {
			continue
		}
		status := strings.TrimSuffix(parts[1], "]")
		result = append(result, domain.TGValidationResult{Username: parts[0], Status: domain.TGStatus(status)})
	}
	return result
}
func parseTime(raw string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		if value, err := time.Parse(layout, raw); err == nil {
			return value
		}
	}
	return time.Time{}
}
