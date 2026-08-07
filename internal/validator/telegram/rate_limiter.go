package telegram

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/gotd/td/tgerr"
)

type RateLimiter struct {
	mu       sync.Mutex
	next     time.Time
	min, max time.Duration
	random   *rand.Rand
}

func NewRateLimiter(minDelay, maxDelay time.Duration) *RateLimiter {
	if minDelay == 0 && maxDelay == 0 {
		minDelay, maxDelay = 500*time.Millisecond, time.Second
	}
	if minDelay < 0 {
		minDelay = 0
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	return &RateLimiter{min: minDelay, max: maxDelay, random: rand.New(rand.NewSource(time.Now().UnixNano()))}
}
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	delta := r.max - r.min
	if delta > 0 {
		delta = time.Duration(r.random.Int63n(int64(delta) + 1))
	}
	wait := time.Until(r.next)
	pacing := r.min + delta
	if pacing > wait {
		wait = pacing
	}
	r.next = time.Now().Add(wait)
	r.mu.Unlock()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (r *RateLimiter) Backoff(delay time.Duration) {
	if delay < 0 {
		return
	}
	r.mu.Lock()
	if candidate := time.Now().Add(delay); candidate.After(r.next) {
		r.next = candidate
	}
	r.mu.Unlock()
}

func HandleFloodWait(ctx context.Context, err error) (bool, error) {
	delay, ok := tgerr.AsFloodWait(err)
	if !ok {
		return false, err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
		return true, nil
	}
}
