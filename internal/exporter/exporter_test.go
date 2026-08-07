package exporter

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/example/glukoza/internal/domain"
	"github.com/xuri/excelize/v2"
)

func testLeads() []*domain.Lead {
	return []*domain.Lead{{ID: "lead-1", CompanyName: "Acme", SourceURL: "https://acme.example", Contacts: domain.ContactInfo{Emails: []string{"one@acme.example", "two@acme.example"}, Telegram: []string{"acme"}}, TGResults: []domain.TGValidationResult{{Username: "acme", Status: domain.TGStatusValid, CheckedAt: time.Unix(0, 0).UTC()}}, IsCIS: true, CISReason: "test reason", CreatedAt: time.Unix(0, 0).UTC()}}
}

func TestExportersWriteFlattenedOutput(t *testing.T) {
	directory := t.TempDir()
	leads := testLeads()
	csvPath := filepath.Join(directory, "leads.csv")
	if err := (&CSVExporter{}).Export(context.Background(), leads, csvPath); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(file).ReadAll()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][0] != "ID" || rows[1][3] != "one@acme.example, two@acme.example" || rows[1][5] != "@acme:[VALID]" {
		t.Fatalf("unexpected CSV rows: %#v", rows)
	}

	jsonPath := filepath.Join(directory, "leads.json")
	if err := (&JSONExporter{}).Export(context.Background(), leads, jsonPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []*domain.Lead
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, leads) {
		t.Fatalf("JSON round trip mismatch: %#v", decoded)
	}

	xlsxPath := filepath.Join(directory, "leads.xlsx")
	if err := (&ExcelExporter{}).Export(context.Background(), leads, xlsxPath); err != nil {
		t.Fatal(err)
	}
	workbook, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	value, err := workbook.GetCellValue("Leads", "F2")
	if err != nil {
		t.Fatal(err)
	}
	if value != "@acme:[VALID]" {
		t.Fatalf("XLSX status = %q", value)
	}
}

func TestExporterFactory(t *testing.T) {
	for _, extension := range []string{".csv", ".json", ".xlsx"} {
		if _, err := NewExporterForFile("output/leads" + extension); err != nil {
			t.Errorf("%s: %v", extension, err)
		}
	}
	if _, err := NewExporterForFile("output/leads.xml"); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}
