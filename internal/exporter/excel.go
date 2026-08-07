package exporter

import (
	"context"
	"fmt"
	"os"

	"github.com/example/glukoza/internal/domain"
	"github.com/xuri/excelize/v2"
)

type ExcelExporter struct{}
type Excel = ExcelExporter

func NewExcelExporter() domain.Exporter { return &ExcelExporter{} }

func (*ExcelExporter) Export(ctx context.Context, leads []*domain.Lead, outputPath string) error {
	if err := os.MkdirAll(directory(outputPath), 0o750); err != nil {
		return err
	}
	file := excelize.NewFile()
	defer file.Close()
	sheet := "Leads"
	index, err := file.NewSheet(sheet)
	if err != nil {
		return fmt.Errorf("create Leads sheet: %w", err)
	}
	file.SetActiveSheet(index)
	_ = file.DeleteSheet("Sheet1")
	headerStyle, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return fmt.Errorf("create header style: %w", err)
	}
	validStyle, err := file.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2F0D9"}, Pattern: 1}})
	if err != nil {
		return err
	}
	invalidStyle, err := file.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#FCE4D6"}, Pattern: 1}})
	if err != nil {
		return err
	}
	headerWidths := make([]int, len(exportHeaders))
	for column, header := range exportHeaders {
		cell, err := excelize.CoordinatesToCellName(column+1, 1)
		if err != nil {
			return err
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			return err
		}
		headerWidths[column] = len([]rune(header))
	}
	if err := file.SetRowStyle(sheet, 1, 1, headerStyle); err != nil {
		return err
	}
	for row, lead := range leads {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if lead == nil {
			continue
		}
		values := flattenLead(lead)
		for column, value := range values {
			cell, err := excelize.CoordinatesToCellName(column+1, row+2)
			if err != nil {
				return err
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return err
			}
			if width := len([]rune(value)); width > headerWidths[column] {
				headerWidths[column] = width
			}
		}
		status := lead.TGResults
		if len(status) > 0 {
			style := invalidStyle
			if status[0].Status == domain.TGStatusValid {
				style = validStyle
			}
			cell, _ := excelize.CoordinatesToCellName(6, row+2)
			if err := file.SetCellStyle(sheet, cell, cell, style); err != nil {
				return err
			}
		}
	}
	for column, width := range headerWidths {
		name, _ := excelize.ColumnNumberToName(column + 1)
		if width < 12 {
			width = 12
		}
		if width > 60 {
			width = 60
		}
		if err := file.SetColWidth(sheet, name, name, float64(width+2)); err != nil {
			return err
		}
	}
	if err := file.SaveAs(outputPath); err != nil {
		return fmt.Errorf("save xlsx: %w", err)
	}
	return nil
}
