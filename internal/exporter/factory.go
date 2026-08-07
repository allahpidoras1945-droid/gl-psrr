package exporter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/example/glukoza/internal/domain"
)

func NewExporterForFile(outputPath string) (domain.Exporter, error) {
	switch strings.ToLower(filepath.Ext(outputPath)) {
	case ".csv":
		return NewCSVExporter(), nil
	case ".json":
		return NewJSONExporter(), nil
	case ".xlsx":
		return NewExcelExporter(), nil
	default:
		return nil, fmt.Errorf("unsupported export format extension: %s", filepath.Ext(outputPath))
	}
}
