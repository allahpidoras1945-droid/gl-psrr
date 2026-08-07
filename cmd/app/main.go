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

	"github.com/example/glukoza/internal/domain"
	"github.com/example/glukoza/internal/exporter"
	"github.com/example/glukoza/internal/filter"
	"github.com/example/glukoza/internal/pipeline"
	"github.com/example/glukoza/internal/scraper"
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
	inputPath := flag.String("input", "", "newline-delimited target URL file")
	categories := flag.String("categories", "", "comma-separated category URLs to crawl")
	outputPath := flag.String("output", envOrDefault("OUTPUT_PATH", "leads_export.xlsx"), "output path (.csv, .json, or .xlsx)")
	workers := flag.Int("workers", envIntOrDefault("WORKERS", 20), "number of concurrent workers")
	tgAppID := flag.Int("tg-appid", envIntOrDefault("TG_APP_ID", 0), "Telegram API application ID")
	tgAppHash := flag.String("tg-apphash", os.Getenv("TG_APP_HASH"), "Telegram API application hash")
	sessionPath := flag.String("session", "data/tg_session.json", "Telegram session file")
	flag.Parse()
	if *workers < 1 {
		return fmt.Errorf("workers must be positive")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var sources []string
	var err error
	if strings.TrimSpace(*categories) != "" {
		categoryURLs := splitValues(*categories)
		crawler := scraper.NewCategoryCrawler(scraper.ScraperConfig{UserAgent: "glukoza-lead-parser/1.0", Timeout: 15 * time.Second})
		sources, err = crawler.DiscoverCardURLs(ctx, categoryURLs)
		if err != nil {
			return fmt.Errorf("discover category cards: %w", err)
		}
		log.Printf("discovered %d card URLs from %d categories", len(sources), len(categoryURLs))
	} else if strings.TrimSpace(*inputPath) != "" {
		sources, err = readLines(*inputPath)
		if err != nil {
			return fmt.Errorf("read input URLs: %w", err)
		}
	} else {
		return fmt.Errorf("provide -categories URL1,URL2 or -input urls.txt")
	}
	scraperEngine := scraper.NewScraperFactory(scraper.ScraperConfig{UserAgent: "glukoza-lead-parser/1.0", Timeout: 15 * time.Second, MaxDepth: 1, Concurrency: *workers}, scraper.NewRegexExtractor())
	cisFilter := filter.NewCISFilter()
	deduplicator := filter.NewDeduplicator()
	var validator domain.TGValidator
	if *tgAppID != 0 || strings.TrimSpace(*tgAppHash) != "" {
		if *tgAppID == 0 || strings.TrimSpace(*tgAppHash) == "" {
			return fmt.Errorf("-tg-appid and -tg-apphash must be provided together")
		}
		tgClient, createErr := telegram.NewTGClient(telegram.ValidatorConfig{AppID: *tgAppID, AppHash: *tgAppHash, SessionPath: *sessionPath, MinDelay: 500 * time.Millisecond, MaxDelay: time.Second})
		if createErr != nil {
			return fmt.Errorf("initialize Telegram client: %w", createErr)
		}
		defer tgClient.Close()
		if startErr := tgClient.Start(ctx); startErr != nil {
			log.Printf("Telegram validator unavailable; continuing with SKIPPED results: %v", startErr)
		} else if authErr := tgClient.EnsureAuthenticated(ctx); authErr != nil {
			return fmt.Errorf("authenticate Telegram client: %w", authErr)
		}
		validator = tgClient
	} else {
		validator = telegram.NewClient(false, *sessionPath, 500*time.Millisecond, time.Second)
	}

	engine := pipeline.NewManager(scraperEngine, cisFilter, deduplicator, validator, *workers)
	started := time.Now()
	leads, runErr := engine.Run(ctx, sources)
	elapsed := time.Since(started)
	log.Printf("pipeline completed: %d leads, %d sources, %s", len(leads), len(sources), elapsed)
	if runErr != nil {
		log.Printf("pipeline completed with recoverable errors: %v", runErr)
	}
	exporterEngine, err := exporter.NewExporterForFile(*outputPath)
	if err != nil {
		return err
	}
	if err := exporterEngine.Export(ctx, leads, *outputPath); err != nil {
		return fmt.Errorf("export leads: %w", err)
	}
	log.Printf("exported %d leads to %s", len(leads), *outputPath)
	return nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	lines := make([]string, 0, 1024)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func splitValues(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}
