package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Workers         int
	RequestTimeout  time.Duration
	UserAgent       string
	CISOnly         bool
	TelegramEnabled bool
	SessionPath     string
	MinDelay        time.Duration
	MaxDelay        time.Duration
	OutputFormat    string
	OutputPath      string
}

func Default() Config {
	return Config{Workers: 16, RequestTimeout: 15 * time.Second, UserAgent: "glukoza-lead-parser/1.0", SessionPath: "data/telegram.session", MinDelay: 500 * time.Millisecond, MaxDelay: time.Second, OutputFormat: "json", OutputPath: "output/leads.json"}
}

// Load reads simple key/value YAML and allows GLUKOZA_* environment overrides.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasSuffix(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		}
	}
	if value := os.Getenv("GLUKOZA_WORKERS"); value != "" {
		values["workers"] = value
	}
	if value := os.Getenv("GLUKOZA_OUTPUT"); value != "" {
		values["output"] = value
	}
	var parseErr error
	if value, ok := values["workers"]; ok {
		cfg.Workers, parseErr = strconv.Atoi(value)
		if parseErr != nil {
			return cfg, fmt.Errorf("workers: %w", parseErr)
		}
	}
	if cfg.Workers < 1 {
		return cfg, fmt.Errorf("workers must be positive")
	}
	if value, ok := values["request_timeout"]; ok {
		cfg.RequestTimeout, parseErr = time.ParseDuration(value)
		if parseErr != nil {
			return cfg, fmt.Errorf("request_timeout: %w", parseErr)
		}
	}
	if value, ok := values["user_agent"]; ok {
		cfg.UserAgent = value
	}
	if value, ok := values["cis_only"]; ok {
		cfg.CISOnly = strings.EqualFold(value, "true")
	}
	if value, ok := values["enabled"]; ok {
		cfg.TelegramEnabled = strings.EqualFold(value, "true")
	}
	if value, ok := values["session_path"]; ok {
		cfg.SessionPath = value
	}
	if value, ok := values["min_delay"]; ok {
		cfg.MinDelay, parseErr = time.ParseDuration(value)
		if parseErr != nil {
			return cfg, fmt.Errorf("min_delay: %w", parseErr)
		}
	}
	if value, ok := values["max_delay"]; ok {
		cfg.MaxDelay, parseErr = time.ParseDuration(value)
		if parseErr != nil {
			return cfg, fmt.Errorf("max_delay: %w", parseErr)
		}
	}
	if value, ok := values["format"]; ok {
		cfg.OutputFormat = strings.ToLower(value)
	}
	if value, ok := values["output"]; ok {
		cfg.OutputPath = value
	}
	return cfg, nil
}
