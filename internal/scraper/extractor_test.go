package scraper

import (
	"reflect"
	"testing"
)

func TestRegexExtractorCleansAndClassifiesContacts(t *testing.T) {
	input := `EMAIL@Example.com, logo@2x.png, hello@example.org. https://t.me/Useful_Name, @Useful_Name, @share, telegram.me/addstickers. linkedin.com/pub/person-1, x.com/intent, x.com/Brand_Name. skype:alice, live:bob. discord.gg/InviteCode and alice#1234.`
	got := (&RegexExtractor{}).ExtractContacts(input)
	want := struct {
		emails, telegram, linkedin, twitter, skype, discord []string
	}{
		emails:   []string{"email@example.com", "hello@example.org"},
		telegram: []string{"useful_name"},
		linkedin: []string{"linkedin.com/pub/person-1"},
		twitter:  []string{"brand_name"},
		skype:    []string{"skype:alice", "live:bob"},
		discord:  []string{"discord.gg/InviteCode", "alice#1234"},
	}
	if !reflect.DeepEqual(got.Emails, want.emails) || !reflect.DeepEqual(got.Telegram, want.telegram) || !reflect.DeepEqual(got.LinkedIn, want.linkedin) || !reflect.DeepEqual(got.Twitter, want.twitter) || !reflect.DeepEqual(got.Skype, want.skype) || !reflect.DeepEqual(got.Discord, want.discord) {
		t.Fatalf("unexpected contacts: %#v", got)
	}
}
