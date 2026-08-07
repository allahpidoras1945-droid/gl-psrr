package scraper

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/example/glukoza/internal/domain"
)

type RegexExtractor struct{}

var (
	emailRE         = regexp.MustCompile(`(?i)(?:^|[^a-z0-9._%+\-])([a-z0-9][a-z0-9._%+\-]*@[a-z0-9](?:[a-z0-9\-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9\-]*[a-z0-9])?)+)`)
	telegramRE      = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?(?:t\.me|telegram\.me)/([a-z0-9_]{5,32})|(?:^|[^a-z0-9._%+\-])@([a-z0-9_]{5,32})`)
	linkedinRE      = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?linkedin\.com/(?:in|company|pub)/[a-z0-9_%-]+`)
	twitterRE       = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?(?:twitter\.com|x\.com)/([a-z0-9_]{1,15})`)
	skypeRE         = regexp.MustCompile(`(?i)(?:skype:|live:)[a-z0-9_.-]+`)
	discordInviteRE = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?(?:discord\.gg|discord\.com/invite)/[a-z0-9-]+`)
	discordTagRE    = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([a-z0-9_.-]{2,32}#[0-9]{4})`)
)

var ignoredEmailExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".css", ".js", ".ico"}
var telegramSystemPaths = map[string]struct{}{"share": {}, "joinchat": {}, "addstickers": {}, "context": {}}
var twitterSystemRoutes = map[string]struct{}{"intent": {}, "share": {}, "home": {}, "search": {}}

func NewRegexExtractor() domain.Extractor { return &RegexExtractor{} }

func (e *RegexExtractor) ExtractContacts(raw string) domain.ContactInfo {
	return domain.ContactInfo{Emails: emailMatches(raw), Telegram: telegramMatches(raw), LinkedIn: cleanLinkedIn(raw), Twitter: twitterMatches(raw), Skype: cleanValues(skypeRE.FindAllString(raw, -1), false), Discord: discordMatches(raw)}
}

func emailMatches(raw string) []string {
	result := []string{}
	for _, match := range emailRE.FindAllStringSubmatch(raw, -1) {
		if len(match) < 2 {
			continue
		}
		email := strings.ToLower(cleanToken(match[1]))
		if email == "" || hasIgnoredExtension(email) || strings.Contains(email, "base64") {
			continue
		}
		result = appendUnique(result, email)
	}
	return result
}
func telegramMatches(raw string) []string {
	result := []string{}
	for _, match := range telegramRE.FindAllStringSubmatch(raw, -1) {
		handle := ""
		if len(match) > 1 {
			handle = match[1]
		}
		if handle == "" && len(match) > 2 {
			handle = match[2]
		}
		handle = strings.ToLower(cleanToken(handle))
		if _, blocked := telegramSystemPaths[handle]; handle == "" || blocked {
			continue
		}
		result = appendUnique(result, handle)
	}
	return result
}
func twitterMatches(raw string) []string {
	result := []string{}
	for _, match := range twitterRE.FindAllStringSubmatch(raw, -1) {
		if len(match) < 2 {
			continue
		}
		handle := strings.ToLower(cleanToken(match[1]))
		if _, blocked := twitterSystemRoutes[handle]; handle == "" || blocked || isAffiliateNoise(handle) {
			continue
		}
		result = appendUnique(result, handle)
	}
	return result
}

func cleanLinkedIn(raw string) []string {
	result := []string{}
	for _, value := range linkedinRE.FindAllString(raw, -1) {
		cleaned := cleanToken(value)
		if cleaned == "" || isAffiliateNoise(cleaned) {
			continue
		}
		result = appendUnique(result, cleaned)
	}
	return result
}

func isAffiliateNoise(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "affpaying") || strings.Contains(lower, "affiliatepaying")
}
func discordMatches(raw string) []string {
	result := cleanValues(discordInviteRE.FindAllString(raw, -1), false)
	for _, match := range discordTagRE.FindAllStringSubmatch(raw, -1) {
		if len(match) > 1 {
			result = appendUnique(result, strings.ToLower(cleanToken(match[1])))
		}
	}
	return result
}
func cleanValues(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = cleanToken(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}
func cleanToken(value string) string {
	return strings.TrimFunc(strings.TrimSpace(value), func(r rune) bool { return unicode.IsPunct(r) && r != '_' && r != '#' })
}
func hasIgnoredExtension(value string) bool {
	for _, extension := range ignoredEmailExtensions {
		if strings.HasSuffix(value, extension) {
			return true
		}
	}
	return false
}
func appendUnique(values []string, candidates ...string) []string {
	for _, value := range candidates {
		if strings.TrimSpace(value) == "" {
			continue
		}
		alreadySeen := false
		for _, existing := range values {
			if strings.EqualFold(existing, value) {
				alreadySeen = true
				break
			}
		}
		if !alreadySeen {
			values = append(values, value)
		}
	}
	return values
}
