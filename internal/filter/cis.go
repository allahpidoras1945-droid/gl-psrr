package filter

import (
	"regexp"
	"strings"
	"sync"

	"github.com/example/glukoza/internal/domain"
)

type CIS struct {
	deduplicator *Deduplicator
	mu           sync.Mutex
}

var (
	cisDomains     = []string{".ru", ".by", ".kz", ".su", ".ua", ".rf"}
	cisMailers     = []string{"yandex.", "mail.ru", "bk.ru", "inbox.ru", "list.ru", "rambler.ru", "ukr.net", "gmx.ru"}
	slavicSuffixRE = regexp.MustCompile(`(?i)(?:[a-z]{3,})(?:enko|chuk|vych|wich|ov|ova|ev|eva|in|ina|skaya|skiy|sky|sks)$`)
)

var cisNamePatterns = []string{
	"aleks", "alexander", "alexey", "dmitry", "dmitrii", "sergey", "sergei", "ivan", "vladimir",
	"anna", "ekaterina", "elena", "olga", "natasha", "mikhail", "artem", "pavel", "igor", "yury", "evgeny",
	"viktoriia", "victoria", "vlad", "bogdan", "danylo", "daria",
}

var systemEmailLocalParts = map[string]struct{}{"admin": {}, "info": {}, "support": {}, "contact": {}, "sales": {}, "help": {}, "billing": {}}
var englishInWords = map[string]struct{}{"justin": {}, "main": {}, "fin": {}, "bin": {}, "win": {}, "domain": {}}

func NewCISFilter() domain.Filter { return &CIS{deduplicator: NewDeduplicator()} }

func (*CIS) IsCIS(lead *domain.Lead) (bool, string) {
	if lead == nil {
		return false, "nil lead"
	}
	if reason := checkName(lead.RawName); reason != "" {
		return true, "Name match: " + reason
	}
	if reason := checkName(lead.CompanyName); reason != "" {
		return true, "Company match: " + reason
	}
	for _, email := range lead.Contacts.Emails {
		lower := strings.ToLower(strings.TrimSpace(email))
		parts := strings.SplitN(lower, "@", 2)
		if len(parts) != 2 {
			continue
		}
		for _, suffix := range cisDomains {
			if strings.HasSuffix(parts[1], suffix) {
				return true, "Email CIS TLD match: " + suffix
			}
		}
		for _, mailer := range cisMailers {
			if strings.Contains(parts[1], mailer) {
				return true, "CIS mail provider match: " + mailer
			}
		}
		if _, system := systemEmailLocalParts[parts[0]]; system {
			continue
		}
		if reason := checkName(parts[0]); reason != "" {
			return true, "Email prefix match: " + reason
		}
	}
	for _, handle := range lead.Contacts.Telegram {
		if reason := checkName(handle); reason != "" {
			return true, "Telegram handle match: " + reason
		}
	}
	return false, ""
}

func checkName(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	for _, name := range cisNamePatterns {
		if lower == name || strings.Contains(lower, name) {
			return "Slavic name token [" + name + "]"
		}
	}
	words := strings.FieldsFunc(lower, func(r rune) bool { return r == ' ' || r == '.' || r == '_' || r == '-' || r == '@' })
	for _, word := range words {
		if _, ignored := englishInWords[word]; ignored {
			continue
		}
		if slavicSuffixRE.MatchString(word) {
			return "Slavic surname suffix in [" + word + "]"
		}
	}
	return ""
}

func (f *CIS) IsDuplicate(identifier string) bool {
	f.mu.Lock()
	if f.deduplicator == nil {
		f.deduplicator = NewDeduplicator()
	}
	deduplicator := f.deduplicator
	f.mu.Unlock()
	return deduplicator.IsDuplicate(identifier)
}
