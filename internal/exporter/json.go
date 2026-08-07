package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/example/glukoza/internal/domain"
)

type JSONExporter struct{}
type JSON = JSONExporter

func NewJSONExporter() domain.Exporter { return &JSONExporter{} }

func (JSONExporter) Export(ctx context.Context, leads []*domain.Lead, outputPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	file, err := openOutput(outputPath)
	if err != nil {
		return fmt.Errorf("create JSON file: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(leads); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
func openOutput(path string) (*os.File, error) {
	if err := os.MkdirAll(directory(path), 0o750); err != nil {
		return nil, err
	}
	return os.Create(path)
}
func directory(path string) string {
	index := len(path) - 1
	for index >= 0 && path[index] != os.PathSeparator {
		index--
	}
	if index < 0 {
		return "."
	}
	return path[:index]
}
func join(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ";"
		}
		result += value
	}
	return result
}
