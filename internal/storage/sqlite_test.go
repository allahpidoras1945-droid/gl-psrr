package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/example/glukoza/internal/domain"
)

func TestInitDBSaveLeadUpsertAndGet(t *testing.T) {
	database, err := InitDB(filepath.Join(t.TempDir(), "nested", "leads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	original := &domain.Lead{ID: "lead-1", SourceURL: "https://example.com/lead", CompanyName: "Example", Contacts: domain.ContactInfo{Emails: []string{"first@example.com"}, Telegram: []string{"example"}, LinkedIn: []string{"example-company"}}, TGResults: []domain.TGValidationResult{{Username: "example", Status: domain.TGStatusValid}}, IsCIS: true, CISReason: "test", CreatedAt: time.Now().UTC()}
	if err := database.SaveLead(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	update := &domain.Lead{ID: "different-id", SourceURL: original.SourceURL, CompanyName: "Updated", Contacts: domain.ContactInfo{Emails: []string{"updated@example.com"}}, IsCIS: true, CreatedAt: time.Now().UTC()}
	if err := database.SaveLead(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	leads, err := database.GetLeads(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("lead count = %d", len(leads))
	}
	if leads[0].CompanyName != "Updated" || leads[0].ID != "lead-1" {
		t.Fatalf("upsert result = %#v", leads[0])
	}
	if !reflect.DeepEqual(leads[0].Contacts.Emails, []string{"updated@example.com"}) {
		t.Fatalf("emails = %#v", leads[0].Contacts.Emails)
	}
	if leads[0].Contacts.LinkedIn[0] != "example-company" {
		t.Fatalf("linkedin = %#v", leads[0].Contacts.LinkedIn)
	}
	cisLeads, err := database.GetLeads(context.Background(), true)
	if err != nil || len(cisLeads) != 1 {
		t.Fatalf("CIS query = %v, %#v", err, cisLeads)
	}
}
