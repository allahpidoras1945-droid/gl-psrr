package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/example/glukoza/internal/domain"
)

func TestNormalizeUsername(t *testing.T) {
	cases := map[string]string{"@Example_User": "example_user", "https://t.me/Example_User?start=1": "example_user", "telegram.me/name/path": "name"}
	for input, want := range cases {
		if got := normalizeUsername(input); got != want {
			t.Errorf("normalizeUsername(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDisabledClientSkipsWithoutPacing(t *testing.T) {
	client := NewClient(false, "", time.Second, time.Second)
	started := time.Now()
	result, err := client.ValidateUsername(context.Background(), "@example")
	if err != nil || result.Status != domain.TGStatusSkipped {
		t.Fatalf("got result=%#v err=%v", result, err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("disabled validation should not wait")
	}
}

func TestRateLimiterDefaultRange(t *testing.T) {
	limiter := NewRateLimiter(0, 0)
	started := time.Now()
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if elapsed < 500*time.Millisecond || elapsed > 1200*time.Millisecond {
		t.Fatalf("default delay = %v, want approximately 500ms-1s", elapsed)
	}
}
