package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const blankValue = "（空白）"

// PivotConfig defines the dimensions and calculation selected in the interface.
type PivotConfig struct {
	RowFields    []string `json:"rowFields"`
	ColumnField  string   `json:"columnField"`
	ValueField   string   `json:"valueField"`
	Aggregation  string   `json:"aggregation"`
	ReportPerson string   `json:"reportPerson"`
}

// PivotRow represents one display row in the pivot preview.
type PivotRow struct {
	Labels  []string  `json:"labels"`
	Values  []float64 `json:"values"`
	Total   float64   `json:"total"`
	IsTotal bool      `json:"isTotal"`
}

// PivotResult is shared by the preview, the first worksheet and the overview calculation.
type PivotResult struct {
	RowHeaders    []string   `json:"rowHeaders"`
	ColumnHeaders []string   `json:"columnHeaders"`
	MetricLabel   string     `json:"metricLabel"`
	Rows          []PivotRow `json:"rows"`
	TotalRows     int        `json:"totalRows"`
}

// OverviewRow is one Apple product category in the exported overview.
type OverviewRow struct {
	Category string  `json:"category"`
	Quantity float64 `json:"quantity"`
}

func suggestedPivotConfig(dataset Dataset) PivotConfig {
	rowField := firstMatchingHeader(dataset.Headers, "upc", "型号", "产品", "商品", "product", "model", "item")
	if rowField == "" && len(dataset.Headers) > 0 {
		rowField = dataset.Headers[0]
	}
	rowFields := []string{rowField}
	productField := firstMatchingHeader(dataset.Headers, "商品名称", "产品名称", "product name", "description")
	if productField != "" && productField != rowField {
		rowFields = append(rowFields, productField)
	}
	valueField := firstMatchingHeader(dataset.Headers, "数量", "qty", "quantity", "count", "库存")
	config := PivotConfig{RowFields: rowFields, Aggregation: "count"}
	if valueField != "" {
		config.ValueField = valueField
		config.Aggregation = "sum"
	}
	return config
}

func firstMatchingHeader(headers []string, keywords ...string) string {
	for _, keyword := range keywords {
		for _, header := range headers {
			if strings.Contains(strings.ToLower(header), strings.ToLower(keyword)) {
				return header
			}
		}
	}
	return ""
}

func buildPivot(dataset Dataset, config PivotConfig) (PivotResult, error) {
	indexes := headerIndexes(dataset.Headers)
	if len(config.RowFields) == 0 {
		return PivotResult{}, fmt.Errorf("请至少选择一个行字段")
	}
	rowIndexes := make([]int, len(config.RowFields))
	for index, field := range config.RowFields {
		fieldIndex, ok := indexes[field]
		if !ok {
			return PivotResult{}, fmt.Errorf("未找到行字段 %q", field)
		}
		rowIndexes[index] = fieldIndex
	}
	columnIndex := -1
	if config.ColumnField != "" {
		var ok bool
		columnIndex, ok = indexes[config.ColumnField]
		if !ok {
			return PivotResult{}, fmt.Errorf("未找到列字段 %q", config.ColumnField)
		}
	}
	valueIndex := -1
	if config.Aggregation == "sum" {
		var ok bool
		valueIndex, ok = indexes[config.ValueField]
		if !ok || config.ValueField == "" {
			return PivotResult{}, fmt.Errorf("求和方式需要选择数值字段")
		}
	}

	groups := make(map[string]map[string]float64)
	groupLabels := make(map[string][]string)
	columnNames := make(map[string]string)
	for _, sourceRow := range dataset.Rows {
		labels := make([]string, len(rowIndexes))
		for index, fieldIndex := range rowIndexes {
			labels[index] = visibleValue(cellAt(sourceRow, fieldIndex))
		}
		groupKey := strings.Join(labels, "\x1f")
		columnKey := "__value__"
		columnName := metricName(config)
		if columnIndex >= 0 {
			columnName = visibleValue(cellAt(sourceRow, columnIndex))
			columnKey = columnName
		}
		value := 1.0
		if config.Aggregation == "sum" {
			value = math.Round(parseNumber(cellAt(sourceRow, valueIndex)))
		}
		if _, ok := groups[groupKey]; !ok {
			groups[groupKey] = make(map[string]float64)
			groupLabels[groupKey] = labels
		}
		groups[groupKey][columnKey] += value
		columnNames[columnKey] = columnName
	}

	columnKeys := make([]string, 0, len(columnNames))
	for key := range columnNames {
		columnKeys = append(columnKeys, key)
	}
	sort.Slice(columnKeys, func(left, right int) bool { return columnNames[columnKeys[left]] < columnNames[columnKeys[right]] })
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Slice(groupKeys, func(left, right int) bool {
		return strings.Join(groupLabels[groupKeys[left]], "\x1f") < strings.Join(groupLabels[groupKeys[right]], "\x1f")
	})

	result := PivotResult{
		RowHeaders:    append([]string(nil), config.RowFields...),
		ColumnHeaders: make([]string, len(columnKeys)),
		MetricLabel:   metricName(config),
		Rows:          make([]PivotRow, 0, len(groupKeys)+1),
		TotalRows:     len(dataset.Rows),
	}
	for index, key := range columnKeys {
		result.ColumnHeaders[index] = columnNames[key]
	}
	totalValues := make([]float64, len(columnKeys))
	for _, groupKey := range groupKeys {
		row := PivotRow{Labels: groupLabels[groupKey], Values: make([]float64, len(columnKeys))}
		for columnIndex, columnKey := range columnKeys {
			row.Values[columnIndex] = groups[groupKey][columnKey]
			row.Total += row.Values[columnIndex]
			totalValues[columnIndex] += row.Values[columnIndex]
		}
		result.Rows = append(result.Rows, row)
	}
	totalLabels := make([]string, len(config.RowFields))
	totalLabels[len(totalLabels)-1] = "总数"
	totalRow := PivotRow{Labels: totalLabels, Values: totalValues, IsTotal: true}
	for _, value := range totalValues {
		totalRow.Total += value
	}
	result.Rows = append(result.Rows, totalRow)
	return result, nil
}

func headerIndexes(headers []string) map[string]int {
	result := make(map[string]int, len(headers))
	for index, header := range headers {
		result[header] = index
	}
	return result
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func visibleValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return blankValue
	}
	return value
}

func metricName(config PivotConfig) string {
	if config.Aggregation == "sum" && config.ValueField != "" {
		return config.ValueField
	}
	return "数量"
}

func parseNumber(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, "，", "")
	allowed := make([]rune, 0, len(value))
	for _, character := range value {
		if (character >= '0' && character <= '9') || character == '.' || character == '-' {
			allowed = append(allowed, character)
		}
	}
	parsed, err := strconv.ParseFloat(string(allowed), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func buildOverview(dataset Dataset, config PivotConfig) []OverviewRow {
	indexes := headerIndexes(dataset.Headers)
	valueIndex := -1
	if config.Aggregation == "sum" {
		valueIndex = indexes[config.ValueField]
	}
	categoryTotals := make(map[string]float64)
	for _, row := range dataset.Rows {
		value := 1.0
		if config.Aggregation == "sum" {
			value = math.Round(parseNumber(cellAt(row, valueIndex)))
		}
		categoryTotals[appleCategory(row, dataset.Headers)] += value
	}
	return orderedOverview(categoryTotals)
}

func appleCategory(row []string, headers []string) string {
	parts := make([]string, 0, len(row))
	for index, value := range row {
		header := ""
		if index < len(headers) {
			header = headers[index]
		}
		parts = append(parts, strings.ToLower(header+" "+value))
	}
	content := strings.Join(parts, " ")
	baseCategory := "其他产品"
	switch {
	case strings.Contains(content, "iphone") || strings.Contains(content, "苹果手机"):
		baseCategory = "手机"
	case strings.Contains(content, "ipad") || strings.Contains(content, "平板"):
		baseCategory = "平板"
	case strings.Contains(content, "macbook"), strings.Contains(content, "imac"), strings.Contains(content, "mac mini"), strings.Contains(content, "mac studio"), strings.Contains(content, "苹果电脑"):
		baseCategory = "电脑"
	case strings.Contains(content, "apple watch"), strings.Contains(content, "watch") || strings.Contains(content, "手表"):
		baseCategory = "穿戴设备"
	case strings.Contains(content, "airpods"), strings.Contains(content, "耳机"):
		baseCategory = "音频设备"
	case strings.Contains(content, "apple tv"), strings.Contains(content, "homepod"), strings.Contains(content, "vision"):
		baseCategory = "家庭与影音"
	case strings.Contains(content, "pencil"), strings.Contains(content, "键盘"), strings.Contains(content, "keyboard"), strings.Contains(content, "mouse"), strings.Contains(content, "充电"), strings.Contains(content, "charger"), strings.Contains(content, "case"), strings.Contains(content, "保护壳"), strings.Contains(content, "cable"), strings.Contains(content, "线"):
		baseCategory = "配件"
	}
	if strings.Contains(content, "refurbished") {
		return "翻新" + baseCategory
	}
	return "全新" + baseCategory
}

func orderedOverview(categoryTotals map[string]float64) []OverviewRow {
	baseOrder := []string{"手机", "平板", "电脑", "穿戴设备", "音频设备", "家庭与影音", "配件", "其他产品"}
	result := make([]OverviewRow, 0, len(categoryTotals))
	for _, condition := range []string{"全新", "翻新"} {
		for _, category := range baseOrder {
			name := condition + category
			if value, exists := categoryTotals[name]; exists {
				result = append(result, OverviewRow{Category: name, Quantity: value})
			}
		}
	}
	return result
}
