package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/example/glukoza/internal/domain"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

type ValidatorConfig struct {
	AppID       int
	AppHash     string
	SessionPath string
	MinDelay    time.Duration
	MaxDelay    time.Duration
}
type TGClient struct {
	config   ValidatorConfig
	client   *telegram.Client
	rawAPI   *tg.Client
	limiter  *RateLimiter
	mu       sync.RWMutex
	startMu  sync.Mutex
	done     chan struct{}
	cancel   context.CancelFunc
	startErr error
}

func NewTGClient(cfg ValidatorConfig) (*TGClient, error) {
	if cfg.AppID <= 0 || strings.TrimSpace(cfg.AppHash) == "" {
		return nil, fmt.Errorf("telegram app id and app hash are required")
	}
	storage, err := NewSessionManager(cfg.SessionPath).Storage()
	if err != nil {
		return nil, err
	}
	return &TGClient{config: cfg, client: telegram.NewClient(cfg.AppID, cfg.AppHash, telegram.Options{SessionStorage: storage}), limiter: NewRateLimiter(cfg.MinDelay, cfg.MaxDelay)}, nil
}

func (c *TGClient) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("telegram start context is nil")
	}
	c.startMu.Lock()
	defer c.startMu.Unlock()
	c.mu.RLock()
	if c.rawAPI != nil {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()
	runCtx, cancel := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan struct{})
	c.mu.Lock()
	c.done, c.cancel = done, cancel
	c.mu.Unlock()
	go func() {
		err := c.client.Run(runCtx, func(runCtx context.Context) error {
			c.mu.Lock()
			c.rawAPI = c.client.API()
			close(ready)
			c.mu.Unlock()
			<-runCtx.Done()
			return nil
		})
		c.mu.Lock()
		c.startErr, c.rawAPI = err, nil
		close(done)
		c.mu.Unlock()
	}()
	select {
	case <-ready:
		return nil
	case <-done:
		c.mu.RLock()
		err := c.startErr
		c.mu.RUnlock()
		if err != nil {
			return fmt.Errorf("telegram client stopped during start: %w", err)
		}
		return fmt.Errorf("telegram client stopped during start")
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return fmt.Errorf("telegram client connection timeout")
	}
}

func (c *TGClient) ValidateUsername(ctx context.Context, raw string) (*domain.TGValidationResult, error) {
	username := normalizeUsername(raw)
	result := &domain.TGValidationResult{Username: username, CheckedAt: time.Now().UTC()}
	if username == "" {
		result.Status = domain.TGStatusInvalid
		result.ErrorReason = "empty username"
		return result, nil
	}
	c.mu.RLock()
	api := c.rawAPI
	c.mu.RUnlock()
	if api == nil {
		result.Status = domain.TGStatusSkipped
		result.ErrorReason = "telegram client is not started"
		return result, nil
	}
	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
		if err != nil {
			if retry, floodErr := HandleFloodWait(ctx, err); retry {
				result.Status = domain.TGStatusFloodWait
				continue
			} else if floodErr != nil {
				return classifyError(result, floodErr), nil
			}
		}
		for _, candidate := range resolved.Users {
			user, ok := candidate.(*tg.User)
			if !ok || user == nil {
				continue
			}
			result.UserID, result.IsBot, result.IsDeleted, result.WasVerified = user.ID, user.Bot, user.Deleted, user.Verified
			if user.Deleted {
				result.Status = domain.TGStatusDeleted
				result.ErrorReason = "Telegram account is marked as deleted"
				return result, nil
			}
			result.Status = domain.TGStatusValid
			return result, nil
		}
		result.Status = domain.TGStatusNotFound
		return result, nil
	}
}

func classifyError(result *domain.TGValidationResult, err error) *domain.TGValidationResult {
	result.ErrorReason = err.Error()
	switch {
	case strings.Contains(result.ErrorReason, "USERNAME_NOT_OCCUPIED"):
		result.Status = domain.TGStatusNotFound
		result.ErrorReason = "username is not occupied; deletion cannot be confirmed by username"
	case strings.Contains(result.ErrorReason, "USERNAME_INVALID"):
		result.Status = domain.TGStatusInvalid
	default:
		result.Status = domain.TGStatusInvalid
	}
	return result
}
func (c *TGClient) Close() error {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	done := c.done
	c.mu.Unlock()
	if done != nil {
		<-done
	}
	return nil
}
func normalizeUsername(input string) string {
	cleaned := strings.TrimSpace(strings.ToLower(input))
	for _, prefix := range []string{"https://", "http://"} {
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}
	cleaned = strings.TrimPrefix(cleaned, "www.")
	for _, prefix := range []string{"t.me/", "telegram.me/", "telegram.dog/", "@"} {
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}
	if index := strings.IndexAny(cleaned, "/?# "); index >= 0 {
		cleaned = cleaned[:index]
	}
	return strings.TrimSpace(cleaned)
}

type Client struct {
	enabled bool
	limiter *RateLimiter
}

func NewClient(enabled bool, _ string, minDelay, maxDelay time.Duration) *Client {
	return &Client{enabled: enabled, limiter: NewRateLimiter(minDelay, maxDelay)}
}
func (c *Client) ValidateUsername(ctx context.Context, raw string) (*domain.TGValidationResult, error) {
	result := &domain.TGValidationResult{Username: normalizeUsername(raw), Status: domain.TGStatusSkipped, CheckedAt: time.Now().UTC(), ErrorReason: "Telegram MTProto is not configured"}
	if !c.enabled {
		result.ErrorReason = "Telegram validation is disabled"
	}
	if !c.enabled {
		return result, nil
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
func (*Client) Close() error { return nil }
