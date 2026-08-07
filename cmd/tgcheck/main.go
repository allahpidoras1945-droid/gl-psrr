package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/example/glukoza/internal/validator/telegram"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	usernamesFlag := flag.String("usernames", "", "comma-separated usernames")
	inputFile := flag.String("file", "", "file with one username per line")
	appID := flag.Int("appid", 0, "Telegram API App ID")
	appHash := flag.String("apphash", "", "Telegram API App Hash")
	sessionPath := flag.String("session", "data/tg_session.json", "persistent session path")
	flag.Parse()
	if *appID == 0 {
		if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("TG_APP_ID"))); err == nil {
			*appID = value
		}
	}
	if *appHash == "" {
		*appHash = strings.TrimSpace(os.Getenv("TG_APP_HASH"))
	}
	if *appID <= 0 || *appHash == "" {
		return fmt.Errorf("Telegram credentials missing: use -appid/-apphash or TG_APP_ID/TG_APP_HASH")
	}
	targets, err := readTargets(*usernamesFlag, *inputFile)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no usernames provided; use -usernames or -file")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := telegram.NewTGClient(telegram.ValidatorConfig{AppID: *appID, AppHash: *appHash, SessionPath: *sessionPath, MinDelay: 500 * time.Millisecond, MaxDelay: time.Second})
	if err != nil {
		return fmt.Errorf("initialize Telegram client: %w", err)
	}
	defer client.Close()
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start Telegram client: %w", err)
	}
	if err := client.EnsureAuthenticated(ctx); err != nil {
		return err
	}
	log.Printf("validating %d usernames", len(targets))
	for _, target := range targets {
		result, validateErr := client.ValidateUsername(ctx, target)
		if validateErr != nil {
			log.Printf("%s: %v", target, validateErr)
			continue
		}
		fmt.Printf("Handle: %-24s Status: %-10s UserID: %-12d Bot: %-5t Verified: %-5t Deleted: %t\n", result.Username, result.Status, result.UserID, result.IsBot, result.WasVerified, result.IsDeleted)
	}
	return nil
}

func readTargets(csvValues, path string) ([]string, error) {
	values := make([]string, 0)
	for _, value := range strings.Split(csvValues, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	if path == "" {
		return values, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open username file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value != "" && !strings.HasPrefix(value, "#") {
			values = append(values, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read username file: %w", err)
	}
	return values, nil
}
