package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

// Dataset is the in-memory representation of the first worksheet in an imported file.
type Dataset struct {
	Path     string
	FileName string
	Headers  []string
	Rows     [][]string
}

// ImportResult contains only the metadata the interface needs after a file is loaded.
type ImportResult struct {
	FileName        string      `json:"fileName"`
	Headers         []string    `json:"headers"`
	NumericFields   []string    `json:"numericFields"`
	RowCount        int         `json:"rowCount"`
	SuggestedConfig PivotConfig `json:"suggestedConfig"`
}

func loadDataset(path string) (Dataset, error) {
	extension := strings.ToLower(filepath.Ext(path))
	var rows [][]string
	var err error

	switch extension {
	case ".xls":
		rows, err = readXLS(path)
	case ".xlsx":
		rows, err = readXLSX(path)
	default:
		return Dataset{}, fmt.Errorf("仅支持 .xls 和 .xlsx 格式")
	}
	if err != nil {
		return Dataset{}, err
	}
	if len(rows) < 2 {
		return Dataset{}, fmt.Errorf("文件至少需要包含一行表头和一行数据")
	}

	headers := uniqueHeaders(rows[0])
	dataRows := make([][]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		normalized := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				normalized[i] = strings.TrimSpace(row[i])
			}
		}
		if !rowIsBlank(normalized) {
			dataRows = append(dataRows, normalized)
		}
	}
	if len(dataRows) == 0 {
		return Dataset{}, fmt.Errorf("未读取到可用于透视的数据行")
	}

	return Dataset{
		Path:     path,
		FileName: filepath.Base(path),
		Headers:  headers,
		Rows:     dataRows,
	}, nil
}

func readXLSX(path string) ([][]string, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开 Excel 文件: %w", err)
	}
	defer func() { _ = file.Close() }()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel 文件中没有工作表")
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取工作表失败: %w", err)
	}
	return rows, nil
}

func readXLS(path string) ([][]string, error) {
	if spreadsheetML, err := readSpreadsheetML(path); err == nil && len(spreadsheetML) > 0 {
		return spreadsheetML, nil
	}
	workbook, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("无法打开 Excel 文件: %w", err)
	}
	if workbook.NumSheets() == 0 {
		return nil, fmt.Errorf("Excel 文件中没有工作表")
	}

	sheet := workbook.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("无法读取第一个工作表")
	}
	rows := make([][]string, 0, int(sheet.MaxRow)+1)
	maxColumns := 0
	for rowIndex := 0; rowIndex <= int(sheet.MaxRow); rowIndex++ {
		row := safeXLSRow(sheet, rowIndex)
		if row == nil {
			continue
		}
		columnCount := row.LastCol()
		if columnCount > maxColumns {
			maxColumns = columnCount
		}
		values := make([]string, columnCount)
		for columnIndex := 0; columnIndex < columnCount; columnIndex++ {
			values[columnIndex] = strings.TrimSpace(row.Col(columnIndex))
		}
		rows = append(rows, values)
	}
	for rowIndex := range rows {
		if len(rows[rowIndex]) < maxColumns {
			rows[rowIndex] = append(rows[rowIndex], make([]string, maxColumns-len(rows[rowIndex]))...)
		}
	}
	return rows, nil
}

func safeXLSRow(sheet *xls.WorkSheet, index int) (row *xls.Row) {
	// The xls package panics for a missing physical row. Empty rows are valid in
	// customer workbooks, so they are treated as absent instead of aborting import.
	defer func() {
		if recover() != nil {
			row = nil
		}
	}()
	return sheet.Row(index)
}

// Excel 2003 XML files are frequently saved with an .xls extension. They need a
// separate reader because they are not BIFF/XLS binaries despite the extension.
func readSpreadsheetML(path string) ([][]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(contents[:min(len(contents), 512)]), "<Workbook") {
		return nil, fmt.Errorf("not SpreadsheetML")
	}
	var workbook spreadsheetMLWorkbook
	if err := xml.Unmarshal(contents, &workbook); err != nil {
		return nil, err
	}
	if len(workbook.Worksheets) == 0 {
		return nil, fmt.Errorf("SpreadsheetML 文件中没有工作表")
	}
	rows := make([][]string, 0, len(workbook.Worksheets[0].Table.Rows))
	maxColumns := 0
	for _, sourceRow := range workbook.Worksheets[0].Table.Rows {
		row := make([]string, 0, len(sourceRow.Cells))
		column := 0
		for _, cell := range sourceRow.Cells {
			if cell.Index > 0 {
				column = cell.Index - 1
			}
			for len(row) <= column {
				row = append(row, "")
			}
			row[column] = strings.TrimSpace(cell.Data)
			column++
		}
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
		rows = append(rows, row)
	}
	for index := range rows {
		if len(rows[index]) < maxColumns {
			rows[index] = append(rows[index], make([]string, maxColumns-len(rows[index]))...)
		}
	}
	return rows, nil
}

type spreadsheetMLWorkbook struct {
	Worksheets []spreadsheetMLWorksheet `xml:"Worksheet"`
}

type spreadsheetMLWorksheet struct {
	Table spreadsheetMLTable `xml:"Table"`
}

type spreadsheetMLTable struct {
	Rows []spreadsheetMLRow `xml:"Row"`
}

type spreadsheetMLRow struct {
	Cells []spreadsheetMLCell `xml:"Cell"`
}

type spreadsheetMLCell struct {
	Index int    `xml:"Index,attr"`
	Data  string `xml:"Data"`
}

func uniqueHeaders(headers []string) []string {
	result := make([]string, len(headers))
	seen := make(map[string]int)
	for index, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			header = fmt.Sprintf("字段 %d", index+1)
		}
		seen[header]++
		if seen[header] > 1 {
			header = fmt.Sprintf("%s (%d)", header, seen[header])
		}
		result[index] = header
	}
	return result
}

func rowIsBlank(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func numericFields(dataset Dataset) []string {
	result := make([]string, 0)
	for columnIndex, header := range dataset.Headers {
		if isIdentifierHeader(header) {
			continue
		}
		nonBlank := 0
		numeric := 0
		for _, row := range dataset.Rows {
			value := strings.TrimSpace(cellAt(row, columnIndex))
			if value == "" {
				continue
			}
			nonBlank++
			if isStrictNumber(value) {
				numeric++
			}
		}
		if nonBlank > 0 && numeric*100 >= nonBlank*85 {
			result = append(result, header)
		}
	}
	return result
}

func isIdentifierHeader(header string) bool {
	header = strings.ToLower(header)
	for _, keyword := range []string{"upc", "imei", "单号", "编号", "id", "编码"} {
		if strings.Contains(header, keyword) {
			return true
		}
	}
	return false
}

func isStrictNumber(value string) bool {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", "")
	value = strings.ReplaceAll(value, "，", "")
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= '0' && character <= '9') || character == '.' {
			continue
		}
		if character == '-' && index == 0 {
			continue
		}
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}
