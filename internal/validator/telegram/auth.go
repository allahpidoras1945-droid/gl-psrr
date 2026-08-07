package telegram

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

type TerminalAuth struct {
	mu     sync.Mutex
	reader *bufio.Reader
}

func NewTerminalAuth() *TerminalAuth { return &TerminalAuth{reader: bufio.NewReader(os.Stdin)} }

func (a *TerminalAuth) read(prompt string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fmt.Print(prompt)
	if a.reader == nil {
		a.reader = bufio.NewReader(os.Stdin)
	}
	value, err := a.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (a *TerminalAuth) Phone(context.Context) (string, error) {
	return a.read("[TG Auth] Phone number: ")
}
func (a *TerminalAuth) Code(context.Context, *tg.AuthSentCode) (string, error) {
	return a.read("[TG Auth] Code: ")
}
func (a *TerminalAuth) Password(context.Context) (string, error) {
	fmt.Print("[TG Auth] 2FA password: ")
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(value)), nil
}
func (*TerminalAuth) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error { return nil }
func (*TerminalAuth) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up is not supported by this CLI")
}

func (c *TGClient) EnsureAuthenticated(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("authentication context is nil")
	}
	status, err := c.client.Auth().Status(ctx)
	if err != nil {
		return fmt.Errorf("check Telegram authorization: %w", err)
	}
	if status.Authorized {
		return nil
	}
	log.Println("Telegram session is not authorized; starting interactive login")
	flow := auth.NewFlow(NewTerminalAuth(), auth.SendCodeOptions{})
	if err := c.client.Auth().IfNecessary(ctx, flow); err != nil {
		return fmt.Errorf("interactive Telegram authentication: %w", err)
	}
	log.Println("Telegram session authenticated and saved")
	return nil
}
