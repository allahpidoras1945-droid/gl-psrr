package email

import (
	"context"
	"testing"
	"time"
)

func TestValidateEmailDomainInvalidAddress(t *testing.T) {
	validator := NewMXValidator(2 * time.Second)
	valid, err := validator.ValidateEmailDomain(context.Background(), "not-an-email")
	if err == nil {
		t.Fatal("expected error for address without a domain")
	}
	if valid {
		t.Fatal("invalid address must not validate")
	}
}

func TestValidateEmailDomainCachesResult(t *testing.T) {
	validator := NewMXValidator(2 * time.Second)
	validator.store("cached.invalid", true)
	valid, err := validator.ValidateEmailDomain(context.Background(), "user@Cached.Invalid")
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("expected cached domain to report valid")
	}
}

func TestValidateEmailDomainSoftPassesOnTimeout(t *testing.T) {
	validator := NewMXValidator(2 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	valid, err := validator.ValidateEmailDomain(ctx, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("expected soft pass when the lookup context is already done")
	}
}
