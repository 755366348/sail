package main

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type InventoryRecord struct {
	OrderNumber string
	UPC         string
	Identifier  string
	ProductName string
	Quantity    float64
}

type OutboundRecord struct {
	BoxNumber      int    `json:"boxNumber"`
	TrackingNumber string `json:"trackingNumber"`
	UPC            string `json:"upc"`
	SN             string `json:"sn"`
	IMEI           string `json:"imei"`
	Identifier     string `json:"identifier"`
	ProductName    string `json:"productName"`
}

type BoxSummary struct {
	Number   int `json:"number"`
	Expected int `json:"expected"`
	Actual   int `json:"actual"`
}

type OutboundData struct {
	Path          string
	FileName      string
	OutboundDate  string
	DeclaredTotal int
	Records       []OutboundRecord
	Boxes         []BoxSummary
	ProductByUPC  map[string]string
}

type OutboundImportResult struct {
	ID            string `json:"id"`
	FileName      string `json:"fileName"`
	DeclaredTotal int    `json:"declaredTotal"`
	DetailTotal   int    `json:"detailTotal"`
	BoxCount      int    `json:"boxCount"`
}

type UnmatchedRecord struct {
	FileName       string `json:"fileName"`
	BoxNumber      int    `json:"boxNumber"`
	TrackingNumber string `json:"trackingNumber"`
	UPC            string `json:"upc"`
	Identifier     string `json:"identifier"`
	ProductName    string `json:"productName"`
}

type ReconciliationResult struct {
	HasOutbound           bool              `json:"hasOutbound"`
	Valid                 bool              `json:"valid"`
	InboundTotal          int               `json:"inboundTotal"`
	OutboundDeclaredTotal int               `json:"outboundDeclaredTotal"`
	OutboundDetailTotal   int               `json:"outboundDetailTotal"`
	MatchedTotal          int               `json:"matchedTotal"`
	RemainingTotal        int               `json:"remainingTotal"`
	Unmatched             []UnmatchedRecord `json:"unmatched"`
	Errors                []string          `json:"errors"`
}

type InventoryReportData struct {
	Inbound        []InventoryRecord
	Outbounds      []OutboundReportData
	Remaining      []InventoryRecord
	Reconciliation ReconciliationResult
}

type OutboundReportData struct {
	Source  OutboundData
	Records []OutboundRecord
}

var (
	boxHeaderPattern = regexp.MustCompile(`^第\s*(\d+)\s*箱[\s　]*[（(]\s*(\d+)\s*件[）)]`)
	sequencePattern  = regexp.MustCompile(`^\d+$`)
)

func parseOutboundFile(path string) (OutboundData, error) {
	sheets, err := loadWorkbookSheets(path)
	if err != nil {
		return OutboundData{}, err
	}
	if len(sheets) < 2 {
		return OutboundData{}, fmt.Errorf("出库文件至少需要包含 Sheet 1 出库信息和 Sheet 2 SN&IMEI 明细")
	}

	outbound := OutboundData{
		Path:         path,
		FileName:     filepath.Base(path),
		ProductByUPC: make(map[string]string),
	}
	if err := parseOutboundSummary(sheets[0].Rows, &outbound); err != nil {
		return OutboundData{}, err
	}
	if err := parseOutboundDetails(sheets[1].Rows, &outbound); err != nil {
		return OutboundData{}, err
	}
	if outbound.DeclaredTotal == 0 {
		return OutboundData{}, fmt.Errorf("未在出库 Sheet 1 找到“出库总数量”")
	}
	if len(outbound.Records) == 0 {
		return OutboundData{}, fmt.Errorf("未在出库 Sheet 2 读取到 IMEI 明细")
	}
	return outbound, nil
}

func parseOutboundSummary(rows [][]string, outbound *OutboundData) error {
	var header map[string]int
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		first := strings.TrimSpace(row[0])
		if strings.Contains(first, "出库总数量") {
			outbound.DeclaredTotal = int(math.Round(parseNumber(firstNonBlank(row[1:]))))
		}
		if strings.Contains(first, "出库日期") {
			outbound.OutboundDate = strings.TrimSpace(firstNonBlank(row[1:]))
		}
		if fields := headerPositions(row); fields["upc"] >= 0 && fields["型号"] >= 0 && fields["数量"] >= 0 {
			header = fields
			continue
		}
		if header == nil || len(row) <= header["upc"] {
			continue
		}
		upc := strings.TrimSpace(cellAt(row, header["upc"]))
		if upc == "" || strings.Contains(upc, "合计") {
			continue
		}
		product := strings.TrimSpace(cellAt(row, header["型号"]))
		if product != "" {
			outbound.ProductByUPC[upc] = product
		}
	}
	return nil
}

func parseOutboundDetails(rows [][]string, outbound *OutboundData) error {
	currentBox := 0
	boxIndex := make(map[int]int)
	var header map[string]int
	for _, row := range rows {
		first := strings.TrimSpace(cellAt(row, 0))
		if matches := boxHeaderPattern.FindStringSubmatch(first); len(matches) == 3 {
			boxNumber, _ := strconv.Atoi(matches[1])
			expected, _ := strconv.Atoi(matches[2])
			currentBox = boxNumber
			boxIndex[currentBox] = len(outbound.Boxes)
			outbound.Boxes = append(outbound.Boxes, BoxSummary{Number: currentBox, Expected: expected})
			header = nil
			continue
		}
		fields := headerPositions(row)
		if fields["序号"] >= 0 && fields["单号"] >= 0 && fields["upc"] >= 0 && fields["imei"] >= 0 {
			header = fields
			continue
		}
		if header == nil || currentBox == 0 || !sequencePattern.MatchString(first) {
			continue
		}
		imei := strings.TrimSpace(cellAt(row, header["imei"]))
		sn := strings.TrimSpace(cellAt(row, header["sn"]))
		identifier := imei
		if identifier == "" {
			identifier = sn
		}
		if identifier == "" {
			return fmt.Errorf("第 %d 箱第 %s 条记录缺少 IMEI", currentBox, first)
		}
		trackingNumber := strings.TrimSpace(cellAt(row, header["单号"]))
		if trackingNumber == "" {
			return fmt.Errorf("第 %d 箱第 %s 条记录缺少单号", currentBox, first)
		}
		record := OutboundRecord{
			BoxNumber:      currentBox,
			TrackingNumber: trackingNumber,
			UPC:            strings.TrimSpace(cellAt(row, header["upc"])),
			SN:             sn,
			IMEI:           imei,
			Identifier:     identifier,
		}
		outbound.Records = append(outbound.Records, record)
		outbound.Boxes[boxIndex[currentBox]].Actual++
	}
	return nil
}

func headerPositions(headers []string) map[string]int {
	positions := map[string]int{"upc": -1, "型号": -1, "数量": -1, "序号": -1, "单号": -1, "sn": -1, "imei": -1}
	for index, header := range headers {
		normalized := strings.ToLower(strings.TrimSpace(header))
		switch normalized {
		case "upc":
			positions["upc"] = index
		case "型号", "商品名称":
			positions["型号"] = index
		case "数量":
			positions["数量"] = index
		case "序号":
			positions["序号"] = index
		case "单号":
			positions["单号"] = index
		case "sn":
			positions["sn"] = index
		case "imei":
			positions["imei"] = index
		}
	}
	return positions
}

func firstNonBlank(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func inboundRecords(dataset Dataset) ([]InventoryRecord, error) {
	indexes := headerIndexes(dataset.Headers)
	for _, field := range []string{"单号", "UPC", "IMEI", "商品名称", "数量"} {
		if _, ok := indexes[field]; !ok {
			return nil, fmt.Errorf("入库表缺少必要字段 %q", field)
		}
	}
	records := make([]InventoryRecord, 0, len(dataset.Rows))
	for _, row := range dataset.Rows {
		orderNumber := strings.TrimSpace(cellAt(row, indexes["单号"]))
		if orderNumber == "" {
			return nil, fmt.Errorf("入库表存在缺少单号的记录")
		}
		identifier := strings.TrimSpace(cellAt(row, indexes["IMEI"]))
		if identifier == "" {
			return nil, fmt.Errorf("入库表存在缺少 IMEI 的记录")
		}
		records = append(records, InventoryRecord{
			OrderNumber: orderNumber,
			UPC:         strings.TrimSpace(cellAt(row, indexes["UPC"])),
			Identifier:  identifier,
			ProductName: strings.TrimSpace(cellAt(row, indexes["商品名称"])),
			Quantity:    math.Round(parseNumber(cellAt(row, indexes["数量"]))),
		})
	}
	return records, nil
}

func reconcileInventory(inbound []InventoryRecord, outbound OutboundData) InventoryReportData {
	return reconcileInventories(inbound, []OutboundData{outbound})
}

func reconcileInventories(inbound []InventoryRecord, outbounds []OutboundData) InventoryReportData {
	report := InventoryReportData{Inbound: inbound}
	result := ReconciliationResult{HasOutbound: len(outbounds) > 0, InboundTotal: quantityOfInbound(inbound)}

	// A business order number is the agreed reconciliation key. One order can
	// legitimately contain several devices, so each order maps to a queue of
	// inbound records. IMEI remains available in raw sheets for reference only.
	inboundByOrderNumber := make(map[string][]int, len(inbound))
	for index, record := range inbound {
		inboundByOrderNumber[record.OrderNumber] = append(inboundByOrderNumber[record.OrderNumber], index)
	}
	matched := make([]bool, len(inbound))
	nextInboundByOrderNumber := make(map[string]int, len(inboundByOrderNumber))
	outboundCountByOrderNumber := make(map[string]int)
	matchedCountByOrderNumber := make(map[string]int)
	for _, source := range outbounds {
		shipment := OutboundReportData{Source: source}
		result.OutboundDeclaredTotal += source.DeclaredTotal
		result.OutboundDetailTotal += len(source.Records)
		for _, box := range source.Boxes {
			if box.Expected != box.Actual {
				result.Errors = append(result.Errors, fmt.Sprintf("%s 第 %d 箱声明 %d 件，实际读取 %d 件", source.FileName, box.Number, box.Expected, box.Actual))
			}
		}
		if len(source.Records) != source.DeclaredTotal {
			result.Errors = append(result.Errors, fmt.Sprintf("%s 出库明细 %d 件与 Sheet 1 出库总数 %d 不一致", source.FileName, len(source.Records), source.DeclaredTotal))
		}
		for _, sourceRecord := range source.Records {
			record := sourceRecord
			outboundCountByOrderNumber[record.TrackingNumber]++
			queue := inboundByOrderNumber[record.TrackingNumber]
			next := nextInboundByOrderNumber[record.TrackingNumber]
			if next < len(queue) {
				incoming := inbound[queue[next]]
				matched[queue[next]] = true
				nextInboundByOrderNumber[record.TrackingNumber] = next + 1
				matchedCountByOrderNumber[record.TrackingNumber]++
				result.MatchedTotal += int(incoming.Quantity)
				record.ProductName = incoming.ProductName
				shipment.Records = append(shipment.Records, record)
			} else {
				record.ProductName = source.ProductByUPC[record.UPC]
				result.Unmatched = append(result.Unmatched, UnmatchedRecord{
					FileName: source.FileName, BoxNumber: record.BoxNumber, TrackingNumber: record.TrackingNumber,
					UPC: record.UPC, Identifier: record.Identifier, ProductName: record.ProductName,
				})
			}
		}
		report.Outbounds = append(report.Outbounds, shipment)
	}
	for index, record := range inbound {
		if !matched[index] {
			report.Remaining = append(report.Remaining, record)
		}
	}
	result.RemainingTotal = quantityOfInbound(report.Remaining)
	orderNumbers := make([]string, 0, len(outboundCountByOrderNumber))
	for orderNumber := range outboundCountByOrderNumber {
		orderNumbers = append(orderNumbers, orderNumber)
	}
	sort.Strings(orderNumbers)
	for _, orderNumber := range orderNumbers {
		outboundCount := outboundCountByOrderNumber[orderNumber]
		if matchedCount := matchedCountByOrderNumber[orderNumber]; matchedCount < outboundCount {
			result.Errors = append(result.Errors, fmt.Sprintf("单号 %s 出库 %d 条，入库可匹配 %d 条", orderNumber, outboundCount, matchedCount))
		}
	}
	if len(result.Unmatched) > 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("%d 条出库单号未在入库表匹配到", len(result.Unmatched)))
	}
	if result.InboundTotal-result.RemainingTotal != result.OutboundDeclaredTotal {
		result.Errors = append(result.Errors, fmt.Sprintf("入库总数 %d - 留仓总数 %d = %d，不等于出库总数 %d", result.InboundTotal, result.RemainingTotal, result.InboundTotal-result.RemainingTotal, result.OutboundDeclaredTotal))
	}
	result.Valid = len(result.Errors) == 0
	report.Reconciliation = result
	return report
}

func quantityOfInbound(records []InventoryRecord) int {
	total := 0
	for _, record := range records {
		total += int(math.Round(record.Quantity))
	}
	return total
}

func inventoryDataset(records []InventoryRecord) Dataset {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{record.UPC, record.ProductName, strconv.Itoa(int(math.Round(record.Quantity)))})
	}
	return Dataset{Headers: []string{"UPC", "商品名称", "数量"}, Rows: rows}
}

func outboundDataset(records []OutboundRecord) Dataset {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{record.UPC, record.ProductName, "1"})
	}
	return Dataset{Headers: []string{"UPC", "商品名称", "数量"}, Rows: rows}
}

func standardPivot(dataset Dataset) (PivotResult, []OverviewRow, error) {
	config := PivotConfig{RowFields: []string{"UPC", "商品名称"}, ValueField: "数量", Aggregation: "sum"}
	pivot, err := buildPivot(dataset, config)
	if err != nil {
		return PivotResult{}, nil, err
	}
	return pivot, buildOverview(dataset, config), nil
}

func inboundRawDataset(records []InventoryRecord) Dataset {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{record.OrderNumber, record.UPC, record.Identifier, record.ProductName, strconv.Itoa(int(record.Quantity))})
	}
	return Dataset{Headers: []string{"单号", "UPC", "IMEI", "商品名称", "数量"}, Rows: rows}
}

func outboundRawDataset(records []OutboundRecord, dateLabel string) Dataset {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{dateLabel, strconv.Itoa(record.BoxNumber), record.TrackingNumber, record.UPC, record.SN, record.IMEI, record.ProductName, dateLabel})
	}
	return Dataset{Headers: []string{dateLabel, "箱号", "单号", "UPC", "SN", "IMEI", "商品名称", dateLabel}, Rows: rows}
}

func remainingRawDataset(records []InventoryRecord, dateLabel, retainedLabel string) Dataset {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{dateLabel, record.OrderNumber, record.UPC, record.Identifier, record.ProductName, strconv.Itoa(int(record.Quantity)), retainedLabel})
	}
	return Dataset{Headers: []string{dateLabel, "单号", "UPC", "IMEI", "商品名称", "数量", retainedLabel}, Rows: rows}
}

func monthDayLabel(value string, fallback time.Time) string {
	if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value)); err == nil {
		return fmt.Sprintf("%d月%d日", parsed.Month(), parsed.Day())
	}
	return fmt.Sprintf("%d月%d日", fallback.Month(), fallback.Day())
}

func retainedLabel(value time.Time) string {
	return fmt.Sprintf("%d.%d留仓", value.Month(), value.Day())
}

func sortUnmatched(records []UnmatchedRecord) {
	sort.Slice(records, func(left, right int) bool {
		if records[left].BoxNumber == records[right].BoxNumber {
			return records[left].TrackingNumber < records[right].TrackingNumber
		}
		return records[left].BoxNumber < records[right].BoxNumber
	})
}
