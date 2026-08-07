package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/example/glukoza/internal/domain"
	"github.com/example/glukoza/internal/exporter"
	"github.com/example/glukoza/internal/filter"
	"github.com/example/glukoza/internal/pipeline"
	"github.com/example/glukoza/internal/scraper"
	"github.com/example/glukoza/internal/validator/telegram"
)

func main() {
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	inputPath := flag.String("input", "urls.txt", "newline-delimited target URL file")
	outputPath := flag.String("output", "leads_export.xlsx", "output path (.csv, .json, or .xlsx)")
	workers := flag.Int("workers", 20, "number of concurrent workers")
	tgAppID := flag.Int("tg-appid", 0, "Telegram API application ID")
	tgAppHash := flag.String("tg-apphash", "", "Telegram API application hash")
	sessionPath := flag.String("session", "data/tg_session.json", "Telegram session file")
	flag.Parse()
	if *workers < 1 {
		return fmt.Errorf("workers must be positive")
	}
	sources, err := readLines(*inputPath)
	if err != nil {
		return fmt.Errorf("read input URLs: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
