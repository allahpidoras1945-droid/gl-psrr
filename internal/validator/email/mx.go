package email

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// MXValidator checks whether an email domain has usable MX records, caching
// results per domain to avoid redundant DNS lookups for repeated domains.
type MXValidator struct {
	resolver *net.Resolver
	timeout  time.Duration
	mu       sync.RWMutex
	cache    map[string]bool
}

func NewMXValidator(timeout time.Duration) *MXValidator {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &MXValidator{resolver: net.DefaultResolver, timeout: timeout, cache: make(map[string]bool)}
}

// ValidateEmailDomain reports whether the email's domain has at least one MX
// record. A DNS timeout is treated as a soft pass to avoid discarding
// potentially valid leads because of a slow resolver.
func (v *MXValidator) ValidateEmailDomain(ctx context.Context, email string) (bool, error) {
	if v == nil {
		return false, fmt.Errorf("MX validator is nil")
	}
	domain := domainOf(email)
	if domain == "" {
		return false, fmt.Errorf("invalid email address %q", email)
	}

	v.mu.RLock()
	cached, ok := v.cache[domain]
	v.mu.RUnlock()
	if ok {
		return cached, nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	records, err := v.resolver.LookupMX(lookupCtx, domain)
	if err != nil {
		if lookupCtx.Err() != nil {
			// Soft pass: a slow DNS server should not disqualify the lead.
			return true, nil
		}
		v.store(domain, false)
		return false, nil
	}
	valid := len(records) > 0
	v.store(domain, valid)
	return valid, nil
}

func (v *MXValidator) store(domain string, valid bool) {
	v.mu.Lock()
	v.cache[domain] = valid
	v.mu.Unlock()
}

func domainOf(email string) string {
	parts := strings.SplitN(strings.TrimSpace(email), "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}
