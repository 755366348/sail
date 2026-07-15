package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// WorkbookSheet is a worksheet represented as plain cell values.
type WorkbookSheet struct {
	Name string
	Rows [][]string
}

func loadWorkbookSheets(path string) ([]WorkbookSheet, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx":
		return readXLSXSheets(path)
	case ".xls":
		contents, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(contents[:min(512, len(contents))]), "<Workbook") {
			return readSpreadsheetMLSheets(string(contents))
		}
		rows, err := readXLS(path)
		if err != nil {
			return nil, err
		}
		return []WorkbookSheet{{Name: "Sheet1", Rows: rows}}, nil
	default:
		return nil, fmt.Errorf("仅支持 .xls 和 .xlsx 格式")
	}
}

func readXLSXSheets(path string) ([]WorkbookSheet, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法打开 Excel 文件: %w", err)
	}
	defer func() { _ = file.Close() }()

	names := file.GetSheetList()
	if len(names) == 0 {
		return nil, fmt.Errorf("Excel 文件中没有工作表")
	}
	sheets := make([]WorkbookSheet, 0, len(names))
	for _, name := range names {
		rows, err := file.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("读取工作表 %q 失败: %w", name, err)
		}
		sheets = append(sheets, WorkbookSheet{Name: name, Rows: rows})
	}
	return sheets, nil
}

var (
	worksheetPattern = regexp.MustCompile(`(?s)<Worksheet\b([^>]*)>(.*?)</Worksheet>`)
	rowPattern       = regexp.MustCompile(`(?s)<Row\b[^>]*>(.*?)</Row>`)
	cellPattern      = regexp.MustCompile(`(?s)<Cell\b([^>]*)>(.*?)</Cell>`)
	dataPattern      = regexp.MustCompile(`(?s)<Data\b[^>]*>(.*?)</Data>`)
	namePattern      = regexp.MustCompile(`\bss:Name="([^"]+)"`)
	indexPattern     = regexp.MustCompile(`\bss:Index="(\d+)"`)
	tagPattern       = regexp.MustCompile(`<[^>]+>`)
)

// readSpreadsheetMLSheets tolerates minor malformed XML produced by the source
// system while still extracting the SpreadsheetML worksheet/row/cell structure.
func readSpreadsheetMLSheets(contents string) ([]WorkbookSheet, error) {
	matches := worksheetPattern.FindAllStringSubmatch(contents, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("SpreadsheetML 文件中没有工作表")
	}
	sheets := make([]WorkbookSheet, 0, len(matches))
	for index, match := range matches {
		name := fmt.Sprintf("Sheet%d", index+1)
		if nameMatch := namePattern.FindStringSubmatch(match[1]); len(nameMatch) == 2 {
			name = html.UnescapeString(nameMatch[1])
		}
		sheets = append(sheets, WorkbookSheet{Name: name, Rows: parseSpreadsheetRows(match[2])})
	}
	return sheets, nil
}

func parseSpreadsheetRows(contents string) [][]string {
	rowMatches := rowPattern.FindAllStringSubmatch(contents, -1)
	rows := make([][]string, 0, len(rowMatches))
	maxColumns := 0
	for _, rowMatch := range rowMatches {
		row := make([]string, 0)
		column := 0
		for _, cellMatch := range cellPattern.FindAllStringSubmatch(rowMatch[1], -1) {
			if indexMatch := indexPattern.FindStringSubmatch(cellMatch[1]); len(indexMatch) == 2 {
				if parsed, err := strconv.Atoi(indexMatch[1]); err == nil && parsed > 0 {
					column = parsed - 1
				}
			}
			for len(row) <= column {
				row = append(row, "")
			}
			value := ""
			if dataMatch := dataPattern.FindStringSubmatch(cellMatch[2]); len(dataMatch) == 2 {
				value = html.UnescapeString(strings.TrimSpace(tagPattern.ReplaceAllString(dataMatch[1], "")))
			}
			row[column] = value
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
	return rows
}
