package exporter

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/example/glukoza/internal/domain"
)

type CSVExporter struct{}
type CSV = CSVExporter

func NewCSVExporter() domain.Exporter { return &CSVExporter{} }

func (*CSVExporter) Export(ctx context.Context, leads []*domain.Lead, outputPath string) error {
	file, err := openOutput(outputPath)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()
	buffered := bufio.NewWriter(file)
	writer := csv.NewWriter(buffered)
	if err := writer.Write(exportHeaders); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, lead := range leads {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if lead == nil {
			continue
		}
		if err := writer.Write(flattenLead(lead)); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		return fmt.Errorf("flush CSV buffer: %w", err)
	}
	return nil
}

var exportHeaders = []string{"ID", "Company Name", "Source URL", "Emails", "Telegram Handles", "TG Validation Status", "LinkedIn", "Skype", "Discord", "Twitter", "Is CIS", "CIS Reason", "Created At"}

func flattenLead(lead *domain.Lead) []string {
	return []string{lead.ID, lead.CompanyName, lead.SourceURL, strings.Join(lead.Contacts.Emails, ", "), strings.Join(lead.Contacts.Telegram, ", "), formatTGStatuses(lead.TGResults), strings.Join(lead.Contacts.LinkedIn, ", "), strings.Join(lead.Contacts.Skype, ", "), strings.Join(lead.Contacts.Discord, ", "), strings.Join(lead.Contacts.Twitter, ", "), strconv.FormatBool(lead.IsCIS), lead.CISReason, lead.CreatedAt.Format("2006-01-02 15:04:05")}
}

func formatTGStatuses(results []domain.TGValidationResult) string {
	if len(results) == 0 {
		return "N/A"
	}
	values := make([]string, 0, len(results))
	for _, result := range results {
		values = append(values, fmt.Sprintf("@%s:[%s]", result.Username, result.Status))
	}
	return strings.Join(values, ", ")
}
