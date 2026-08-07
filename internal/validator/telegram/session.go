package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gotd/td/session"
)

type SessionManager struct{ path string }

func NewSessionManager(path string) *SessionManager { return &SessionManager{path: path} }
func (s *SessionManager) Storage() (session.Storage, error) {
	if s.path == "" {
		return nil, fmt.Errorf("session path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}
	return &lockedStorage{storage: &session.FileStorage{Path: s.path}}, nil
}

type lockedStorage struct {
	mu      sync.Mutex
	storage session.Storage
}

func (s *lockedStorage) LoadSession(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storage.LoadSession(ctx)
}
func (s *lockedStorage) StoreSession(ctx context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storage.StoreSession(ctx, data)
}
