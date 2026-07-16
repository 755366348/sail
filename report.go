package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// SaveResult identifies the workbook created at the user-selected location.
type SaveResult struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
}

func writeReport(pivot PivotResult, overview []OverviewRow, reportPerson string) (SaveResult, error) {
	desktop, err := desktopDirectory()
	if err != nil {
		return SaveResult{}, err
	}
	fileName := fmt.Sprintf("Apple_数据汇总_%s.xlsx", time.Now().Format("20060102"))
	path := availablePath(filepath.Join(desktop, fileName))
	reportTitle := formatReportTitle(time.Now(), reportPerson)
	if err := writeReportFile(pivot, overview, reportTitle, path); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Path: path, FileName: filepath.Base(path)}, nil
}

func formatReportTitle(reportTime time.Time, reportPerson string) string {
	return fmt.Sprintf("%s %s", reportTime.Format("1.2"), strings.TrimSpace(reportPerson))
}

func writeReportFile(pivot PivotResult, overview []OverviewRow, reportTitle string, path string) error {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()

	pivotSheet := "数据透视"
	overviewSheet := "数据概览"
	book.SetSheetName("Sheet1", pivotSheet)
	if _, err := book.NewSheet(overviewSheet); err != nil {
		return fmt.Errorf("创建概览工作表失败: %w", err)
	}
	styles, err := createReportStyles(book)
	if err != nil {
		return fmt.Errorf("创建报表样式失败: %w", err)
	}
	if err := writePivotSheet(book, pivotSheet, pivot, reportTitle, styles); err != nil {
		return err
	}
	if err := writeOverviewSheet(book, overviewSheet, overview, reportTitle, styles); err != nil {
		return err
	}
	book.SetActiveSheet(0)
	if err := book.SaveAs(path); err != nil {
		return fmt.Errorf("保存报表失败: %w", err)
	}
	return nil
}

func writeInventoryReport(report InventoryReportData, reportPerson, path string) (SaveResult, error) {
	if err := writeInventoryReportFile(report, reportPerson, path); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Path: path, FileName: filepath.Base(path)}, nil
}

func writeInventoryReportFile(report InventoryReportData, reportPerson, path string) error {
	now := time.Now()
	reportTime := now
	if len(report.Outbounds) > 0 {
		if parsed, parseErr := time.Parse("2006-01-02", strings.TrimSpace(report.Outbounds[0].Source.OutboundDate)); parseErr == nil {
			reportTime = parsed
		}
	}
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	styles, err := createReportStyles(book)
	if err != nil {
		return fmt.Errorf("创建报表样式失败: %w", err)
	}
	title := formatReportTitle(reportTime, reportPerson)

	inboundPivot, inboundOverview, err := standardPivot(inventoryDataset(report.Inbound))
	if err != nil {
		return err
	}
	if err := writeInventorySheetSet(book, "Sheet1", "入库表", "入库-透视", "入库-概览", inboundRawDataset(report.Inbound), inboundPivot, inboundOverview, title, styles); err != nil {
		return err
	}
	for index, shipment := range report.Outbounds {
		shipmentTime := reportTime
		if parsed, parseErr := time.Parse("2006-01-02", strings.TrimSpace(shipment.Source.OutboundDate)); parseErr == nil {
			shipmentTime = parsed
		}
		dateLabel := monthDayLabel(shipment.Source.OutboundDate, shipmentTime)
		outboundPivot, outboundOverview, err := standardPivot(outboundDataset(shipment.Records))
		if err != nil {
			return err
		}
		suffix := ""
		if index > 0 {
			suffix = fmt.Sprintf("-%d", index+1)
		}
		shipmentTitle := formatReportTitle(shipmentTime, reportPerson)
		if err := writeInventorySheetSet(book, "", "出库表"+suffix, "出库-透视"+suffix, "出库-概览"+suffix, outboundRawDataset(shipment.Records, dateLabel), outboundPivot, outboundOverview, shipmentTitle, styles); err != nil {
			return err
		}
	}
	if len(report.Outbounds) > 0 {
		remainingPivot, remainingOverview, err := standardPivot(inventoryDataset(report.Remaining))
		if err != nil {
			return err
		}
		dateLabel := monthDayLabel(report.Outbounds[len(report.Outbounds)-1].Source.OutboundDate, reportTime)
		if err := writeInventorySheetSet(book, "", "留仓表", "留仓-透视", "留仓-概览", remainingRawDataset(report.Remaining, dateLabel, retainedLabel(reportTime)), remainingPivot, remainingOverview, title, styles); err != nil {
			return err
		}
	}
	if len(report.Reconciliation.Unmatched) > 0 {
		unmatchedSheet := "不匹配项-单号"
		if report.MatchMode == matchByIMEI {
			unmatchedSheet = "不匹配项-IMEI"
		}
		if _, err := book.NewSheet(unmatchedSheet); err != nil {
			return err
		}
		if err := writeRawSheet(book, unmatchedSheet, unmatchedRawDataset(report.Reconciliation.Unmatched), styles); err != nil {
			return err
		}
	}
	book.SetActiveSheet(0)
	if err := book.SaveAs(path); err != nil {
		return fmt.Errorf("保存报表失败: %w", err)
	}
	return nil
}

func writeInventorySheetSet(book *excelize.File, defaultSheet, rawName, pivotName, overviewName string, raw Dataset, pivot PivotResult, overview []OverviewRow, title string, styles reportStyles) error {
	if defaultSheet != "" {
		book.SetSheetName(defaultSheet, rawName)
	} else if _, err := book.NewSheet(rawName); err != nil {
		return err
	}
	if err := writeRawSheet(book, rawName, raw, styles); err != nil {
		return err
	}
	if _, err := book.NewSheet(pivotName); err != nil {
		return err
	}
	if err := writePivotSheet(book, pivotName, pivot, title, styles); err != nil {
		return err
	}
	if _, err := book.NewSheet(overviewName); err != nil {
		return err
	}
	return writeOverviewSheet(book, overviewName, overview, title, styles)
}

func writeRawSheet(book *excelize.File, sheet string, dataset Dataset, styles reportStyles) error {
	if len(dataset.Headers) == 0 {
		return fmt.Errorf("%s 缺少表头", sheet)
	}
	lastColumn := excelColumn(len(dataset.Headers))
	for index, header := range dataset.Headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		book.SetCellValue(sheet, cell, header)
	}
	book.SetCellStyle(sheet, "A1", lastColumn+"1", styles.header)
	book.SetRowHeight(sheet, 1, 24)
	for rowIndex, row := range dataset.Rows {
		excelRow := rowIndex + 2
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, excelRow)
			book.SetCellValue(sheet, cell, value)
		}
		book.SetCellStyle(sheet, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("%s%d", lastColumn, excelRow), styles.cell)
	}
	for columnIndex, header := range dataset.Headers {
		width := max(12, visualWidth(header)+4)
		for _, row := range dataset.Rows {
			width = max(width, visualWidth(cellAt(row, columnIndex))+4)
		}
		width = min(width, 42)
		column := excelColumn(columnIndex + 1)
		book.SetColWidth(sheet, column, column, float64(width))
	}
	return book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

type reportStyles struct {
	title  int
	header int
	cell   int
	number int
	total  int
}

func createReportStyles(book *excelize.File) (reportStyles, error) {
	const reportFont = "宋体"
	const reportFontSize = 10
	border := []excelize.Border{
		{Type: "left", Color: "1A1A1A", Style: 1},
		{Type: "right", Color: "1A1A1A", Style: 1},
		{Type: "top", Color: "1A1A1A", Style: 1},
		{Type: "bottom", Color: "1A1A1A", Style: 1},
	}
	title, err := book.NewStyle(&excelize.Style{
		Border:    border,
		Font:      &excelize.Font{Family: reportFont, Size: reportFontSize},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return reportStyles{}, err
	}
	header, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: reportFont, Size: reportFontSize, Bold: true, Color: "16324F"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"DCECF0"}, Pattern: 1},
		Border:    border,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return reportStyles{}, err
	}
	cell, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: reportFont, Size: reportFontSize},
		Border:    border,
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return reportStyles{}, err
	}
	number, err := book.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: reportFont, Size: reportFontSize},
		Border:       border,
		CustomNumFmt: stringPointer("#,##0"),
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
	})
	if err != nil {
		return reportStyles{}, err
	}
	total, err := book.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Family: reportFont, Size: reportFontSize, Bold: true, Color: "000000"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"00B050"}, Pattern: 1},
		Border:       border,
		CustomNumFmt: stringPointer("#,##0"),
		Alignment:    &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return reportStyles{}, err
	}
	return reportStyles{title: title, header: header, cell: cell, number: number, total: total}, nil
}

func stringPointer(value string) *string {
	return &value
}

func writePivotSheet(book *excelize.File, sheet string, pivot PivotResult, reportTitle string, styles reportStyles) error {
	columnCount := max(1, len(pivot.RowHeaders)+len(pivot.ColumnHeaders))
	lastColumn := excelColumn(columnCount)
	book.SetCellValue(sheet, "A1", reportTitle)
	book.SetCellStyle(sheet, "A1", lastColumn+"1", styles.title)
	book.SetRowHeight(sheet, 1, 28)

	for rowIndex, row := range pivot.Rows {
		excelRow := rowIndex + 2
		if !row.IsTotal {
			for columnIndex, label := range row.Labels {
				cell, _ := excelize.CoordinatesToCellName(columnIndex+1, excelRow)
				book.SetCellValue(sheet, cell, label)
			}
		} else {
			// Keep only the UPC column blank and align the total label with the model name.
			totalLabelColumn := min(2, len(pivot.RowHeaders))
			totalLabelCell, _ := excelize.CoordinatesToCellName(totalLabelColumn, excelRow)
			book.SetCellValue(sheet, totalLabelCell, "总数")
		}
		valueStart := len(pivot.RowHeaders) + 1
		for columnIndex, value := range row.Values {
			cell, _ := excelize.CoordinatesToCellName(valueStart+columnIndex, excelRow)
			book.SetCellValue(sheet, cell, value)
		}
		if row.IsTotal {
			book.SetCellStyle(sheet, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("%s%d", lastColumn, excelRow), styles.total)
		} else {
			labelEnd := excelColumn(len(pivot.RowHeaders))
			book.SetCellStyle(sheet, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("%s%d", labelEnd, excelRow), styles.cell)
			book.SetCellStyle(sheet, fmt.Sprintf("%s%d", excelColumn(valueStart), excelRow), fmt.Sprintf("%s%d", lastColumn, excelRow), styles.number)
		}
	}
	setPivotColumnWidths(book, sheet, pivot)
	return nil
}

func writeOverviewSheet(book *excelize.File, sheet string, overview []OverviewRow, reportTitle string, styles reportStyles) error {
	categoryColumn := 1
	quantityColumn := 2
	lastColumn := excelColumn(quantityColumn)
	book.SetCellValue(sheet, "A1", reportTitle)
	book.SetCellStyle(sheet, "A1", lastColumn+"1", styles.title)
	book.SetRowHeight(sheet, 1, 28)

	total := 0.0
	for index, row := range overview {
		excelRow := index + 2
		categoryCell, _ := excelize.CoordinatesToCellName(categoryColumn, excelRow)
		quantityCell, _ := excelize.CoordinatesToCellName(quantityColumn, excelRow)
		book.SetCellValue(sheet, categoryCell, row.Category)
		book.SetCellValue(sheet, quantityCell, row.Quantity)
		book.SetCellStyle(sheet, fmt.Sprintf("A%d", excelRow), categoryCell, styles.cell)
		book.SetCellStyle(sheet, quantityCell, quantityCell, styles.number)
		total += row.Quantity
	}
	totalRow := len(overview) + 2
	categoryCell, _ := excelize.CoordinatesToCellName(categoryColumn, totalRow)
	quantityCell, _ := excelize.CoordinatesToCellName(quantityColumn, totalRow)
	book.SetCellValue(sheet, categoryCell, "总数")
	book.SetCellValue(sheet, quantityCell, total)
	book.SetCellStyle(sheet, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("%s%d", lastColumn, totalRow), styles.total)

	for column := 1; column <= quantityColumn; column++ {
		name := excelColumn(column)
		width := 16.0
		if column == categoryColumn {
			width = 24
		}
		book.SetColWidth(sheet, name, name, width)
	}
	return nil
}

func setPivotColumnWidths(book *excelize.File, sheet string, pivot PivotResult) {
	for columnIndex := 0; columnIndex < len(pivot.RowHeaders); columnIndex++ {
		width := 12
		for _, row := range pivot.Rows {
			if columnIndex < len(row.Labels) {
				width = max(width, visualWidth(row.Labels[columnIndex])+4)
			}
		}
		width = min(width, 42)
		column := excelColumn(columnIndex + 1)
		book.SetColWidth(sheet, column, column, float64(width))
	}
	for columnIndex := len(pivot.RowHeaders) + 1; columnIndex <= len(pivot.RowHeaders)+len(pivot.ColumnHeaders); columnIndex++ {
		column := excelColumn(columnIndex)
		book.SetColWidth(sheet, column, column, 14)
	}
}

func visualWidth(value string) int {
	width := 0
	for _, character := range value {
		if character > 127 {
			width += 2
		} else {
			width++
		}
	}
	if utf8.RuneCountInString(value) == 0 {
		return 0
	}
	return width
}

func excelColumn(number int) string {
	name, _ := excelize.ColumnNumberToName(number)
	return name
}

func desktopDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户目录: %w", err)
	}
	desktop := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		return "", fmt.Errorf("无法访问桌面目录: %w", err)
	}
	return desktop, nil
}

func availablePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(path, extension)
	for counter := 2; ; counter++ {
		candidate := fmt.Sprintf("%s_%d%s", base, counter, extension)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
