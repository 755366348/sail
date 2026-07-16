package main

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestLoadProvidedXLS(t *testing.T) {
	dataset, err := loadDataset(filepath.Join("a.xls"))
	if err != nil {
		t.Fatalf("loadDataset returned an error: %v", err)
	}
	if len(dataset.Headers) == 0 || len(dataset.Rows) == 0 {
		t.Fatal("expected headers and data rows")
	}
	pivot, err := buildPivot(dataset, suggestedPivotConfig(dataset))
	if err != nil {
		t.Fatalf("buildPivot returned an error: %v", err)
	}
	if len(pivot.Rows) < 2 || !pivot.Rows[len(pivot.Rows)-1].IsTotal {
		t.Fatal("expected a pivot total row")
	}
	t.Logf("headers=%v rows=%d default=%+v", dataset.Headers, len(dataset.Rows), suggestedPivotConfig(dataset))
}

func TestWriteReportFile(t *testing.T) {
	dataset, err := loadDataset("a.xls")
	if err != nil {
		t.Fatalf("loadDataset returned an error: %v", err)
	}
	config := suggestedPivotConfig(dataset)
	pivot, err := buildPivot(dataset, config)
	if err != nil {
		t.Fatalf("buildPivot returned an error: %v", err)
	}
	overview := buildOverview(dataset, config)
	path := filepath.Join(t.TempDir(), "pivot-report.xlsx")
	if err := writeReportFile(pivot, overview, "2026.07.14 测试人", path); err != nil {
		t.Fatalf("writeReportFile returned an error: %v", err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("generated workbook could not be opened: %v", err)
	}
	defer func() { _ = book.Close() }()
	if sheets := book.GetSheetList(); len(sheets) != 2 || sheets[0] != "数据透视" || sheets[1] != "数据概览" {
		t.Fatalf("unexpected worksheet list: %v", sheets)
	}
	if value, err := book.GetCellValue("数据透视", "A1"); err != nil || value != "2026.07.14 测试人" {
		t.Fatalf("expected title in pivot sheet, got %q (err=%v)", value, err)
	}
	if value, err := book.GetCellValue("数据概览", "A1"); err != nil || value != "2026.07.14 测试人" {
		t.Fatalf("expected title in overview sheet, got %q (err=%v)", value, err)
	}
	pivotTotalRow := len(pivot.Rows) + 1
	if value, err := book.GetCellValue("数据透视", "B"+strconv.Itoa(pivotTotalRow)); err != nil || value != "总数" {
		t.Fatalf("expected pivot total label, got %q (err=%v)", value, err)
	}
	overviewTotalRow := len(overview) + 2
	if value, err := book.GetCellValue("数据概览", "A"+strconv.Itoa(overviewTotalRow)); err != nil || value != "总数" {
		t.Fatalf("expected overview total label, got %q (err=%v)", value, err)
	}
	if value, err := book.GetCellValue("数据透视", "C2"); err != nil || strings.Contains(value, ".") {
		t.Fatalf("expected integer pivot quantity, got %q (err=%v)", value, err)
	}
	for _, row := range pivot.Rows {
		for _, value := range row.Values {
			if value != math.Round(value) {
				t.Fatalf("pivot value should be an integer, got %v", value)
			}
		}
	}
	for _, row := range overview {
		if row.Quantity != math.Round(row.Quantity) {
			t.Fatalf("overview value should be an integer, got %v", row.Quantity)
		}
	}
	if title := formatReportTitle(time.Date(2026, time.July, 14, 0, 0, 0, 0, time.Local), "测试人"); title != "7.14 测试人" {
		t.Fatalf("unexpected report title: %q", title)
	}
}

func TestParseOutboundAndReconcileSampleFiles(t *testing.T) {
	outbound, err := parseOutboundFile("b.xls")
	if err != nil {
		t.Fatalf("parseOutboundFile returned an error: %v", err)
	}
	if outbound.DeclaredTotal != 853 || len(outbound.Records) != 853 || len(outbound.Boxes) != 19 {
		t.Fatalf("unexpected outbound structure: total=%d records=%d boxes=%d", outbound.DeclaredTotal, len(outbound.Records), len(outbound.Boxes))
	}
	for _, box := range outbound.Boxes {
		if box.Expected != box.Actual {
			t.Fatalf("box %d expected %d records, got %d", box.Number, box.Expected, box.Actual)
		}
	}
	inboundDataset, err := loadDataset("a.xls")
	if err != nil {
		t.Fatalf("loadDataset returned an error: %v", err)
	}
	inbound, err := inboundRecords(inboundDataset)
	if err != nil {
		t.Fatalf("inboundRecords returned an error: %v", err)
	}
	report := reconcileInventory(inbound, outbound)
	if report.Reconciliation.Valid {
		t.Fatal("sample data should fail reconciliation because it contains unmatched outbound identifiers")
	}
	if report.Reconciliation.InboundTotal != 871 || report.Reconciliation.RemainingTotal != 31 || len(report.Reconciliation.Unmatched) != 13 {
		t.Fatalf("unexpected reconciliation: %+v", report.Reconciliation)
	}
	_, overview, err := standardPivot(inventoryDataset(inbound))
	if err != nil {
		t.Fatalf("standardPivot returned an error: %v", err)
	}
	categories := make(map[string]bool)
	for _, row := range overview {
		categories[row.Category] = true
	}
	if !categories["全新手机"] || !categories["翻新手机"] {
		t.Fatalf("expected new and refurbished phone categories, got %v", categories)
	}
}

func TestReconcileMatchesMultipleRecordsByOrderNumber(t *testing.T) {
	inbound := []InventoryRecord{
		{OrderNumber: "ORDER-001", UPC: "UPC-A", Identifier: "IMEI-IN-1", ProductName: "Phone A", Quantity: 1},
		{OrderNumber: "ORDER-001", UPC: "UPC-B", Identifier: "IMEI-IN-2", ProductName: "Phone B", Quantity: 1},
	}
	outbound := OutboundData{
		FileName:      "outbound.xls",
		DeclaredTotal: 2,
		Records: []OutboundRecord{
			{TrackingNumber: "ORDER-001", UPC: "UPC-A", Identifier: "IMEI-OUT-1"},
			{TrackingNumber: "ORDER-001", UPC: "UPC-B", Identifier: "IMEI-OUT-2"},
		},
	}
	report := reconcileInventory(inbound, outbound)
	if !report.Reconciliation.Valid || report.Reconciliation.MatchedTotal != 2 || report.Reconciliation.RemainingTotal != 0 {
		t.Fatalf("expected order-number matching to consume both records, got %+v", report.Reconciliation)
	}
}

func TestReconcileMatchesByIMEIWhenSelected(t *testing.T) {
	inbound := []InventoryRecord{{OrderNumber: "IN-ORDER", UPC: "UPC-A", Identifier: "IMEI-001", ProductName: "Phone A", Quantity: 1}}
	outbound := OutboundData{
		FileName:      "outbound.xls",
		DeclaredTotal: 1,
		Records:       []OutboundRecord{{TrackingNumber: "OUT-ORDER", UPC: "UPC-A", Identifier: "IMEI-001"}},
	}
	report := reconcileInventoriesWithMatchMode(inbound, []OutboundData{outbound}, string(matchByIMEI))
	if !report.Reconciliation.Valid || report.Reconciliation.MatchedTotal != 1 || report.Reconciliation.RemainingTotal != 0 {
		t.Fatalf("expected IMEI matching to ignore different order numbers, got %+v", report.Reconciliation)
	}
}

func TestWriteInventoryReportIncludesIMEIUnmatchedSheet(t *testing.T) {
	inbound := []InventoryRecord{{OrderNumber: "IN-ORDER", UPC: "UPC-A", Identifier: "IMEI-IN", ProductName: "Phone A", Quantity: 1}}
	outbound := OutboundData{
		FileName:      "outbound.xls",
		DeclaredTotal: 1,
		Records:       []OutboundRecord{{TrackingNumber: "OUT-ORDER", UPC: "UPC-A", Identifier: "IMEI-OUT"}},
	}
	report := reconcileInventoriesWithMatchMode(inbound, []OutboundData{outbound}, string(matchByIMEI))
	if len(report.Reconciliation.Unmatched) != 1 {
		t.Fatalf("expected one unmatched IMEI, got %+v", report.Reconciliation)
	}
	path := filepath.Join(t.TempDir(), "imei-unmatched.xlsx")
	if err := writeInventoryReportFile(report, "测试人", path); err != nil {
		t.Fatalf("writeInventoryReportFile returned an error: %v", err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("generated workbook could not be opened: %v", err)
	}
	defer func() { _ = book.Close() }()
	sheets := strings.Join(book.GetSheetList(), ",")
	if !strings.Contains(sheets, "不匹配项-IMEI") || strings.Contains(sheets, "不匹配项-单号") {
		t.Fatalf("unexpected IMEI unmatched sheet list: %v", book.GetSheetList())
	}
}

func TestWriteInventoryReportFiles(t *testing.T) {
	inboundDataset, err := loadDataset("a.xls")
	if err != nil {
		t.Fatalf("loadDataset returned an error: %v", err)
	}
	inbound, err := inboundRecords(inboundDataset)
	if err != nil {
		t.Fatalf("inboundRecords returned an error: %v", err)
	}
	threeSheetPath := filepath.Join(t.TempDir(), "inbound-only.xlsx")
	if err := writeInventoryReportFile(InventoryReportData{Inbound: inbound}, "测试人", threeSheetPath); err != nil {
		t.Fatalf("writeInventoryReportFile returned an error: %v", err)
	}
	book, err := excelize.OpenFile(threeSheetPath)
	if err != nil {
		t.Fatalf("generated inbound workbook could not be opened: %v", err)
	}
	if sheets := book.GetSheetList(); strings.Join(sheets, ",") != "入库表,入库-透视,入库-概览" {
		t.Fatalf("unexpected inbound sheet list: %v", sheets)
	}
	headers, err := book.GetRows("入库表")
	if err != nil || len(headers) == 0 || strings.Contains(strings.Join(headers[0], ","), "入库时间") {
		t.Fatalf("inbound raw sheet should not include 入库时间: %v", headers)
	}
	_ = book.Close()

	outbound, err := parseOutboundFile("b.xls")
	if err != nil {
		t.Fatalf("parseOutboundFile returned an error: %v", err)
	}
	report := reconcileInventory(inbound, outbound)
	nineSheetPath := filepath.Join(t.TempDir(), "inventory-report.xlsx")
	if err := writeInventoryReportFile(report, "测试人", nineSheetPath); err != nil {
		t.Fatalf("writeInventoryReportFile returned an error: %v", err)
	}
	book, err = excelize.OpenFile(nineSheetPath)
	if err != nil {
		t.Fatalf("generated inventory workbook could not be opened: %v", err)
	}
	defer func() { _ = book.Close() }()
	expected := "入库表,入库-透视,入库-概览,出库表,出库-透视,出库-概览,留仓表,留仓-透视,留仓-概览,不匹配项-单号"
	if sheets := book.GetSheetList(); strings.Join(sheets, ",") != expected {
		t.Fatalf("unexpected inventory sheet list: %v", sheets)
	}
	outboundRows, err := book.GetRows("出库表")
	if err != nil || len(outboundRows) != 841 || outboundRows[0][0] != "7月14日" || outboundRows[0][len(outboundRows[0])-1] != "7月14日" {
		t.Fatalf("unexpected outbound raw sheet: rows=%d headers=%v err=%v", len(outboundRows), outboundRows[0], err)
	}
	remainingRows, err := book.GetRows("留仓表")
	if err != nil || len(remainingRows) != 32 || remainingRows[0][0] != "7月14日" || remainingRows[0][len(remainingRows[0])-1] != "7.14留仓" {
		t.Fatalf("unexpected remaining raw sheet: rows=%d headers=%v err=%v", len(remainingRows), remainingRows[0], err)
	}
	unmatchedRows, err := book.GetRows("不匹配项-单号")
	if err != nil || len(unmatchedRows) != 14 || strings.Join(unmatchedRows[0], ",") != "出库表,箱号,单号,UPC,IMEI,商品名称" {
		t.Fatalf("unexpected unmatched raw sheet: rows=%d headers=%v err=%v", len(unmatchedRows), unmatchedRows[0], err)
	}
}

func TestMultipleOutboundReportsUseOneRemainingSheet(t *testing.T) {
	inboundDataset, err := loadDataset("a.xls")
	if err != nil {
		t.Fatalf("loadDataset returned an error: %v", err)
	}
	inbound, err := inboundRecords(inboundDataset)
	if err != nil {
		t.Fatalf("inboundRecords returned an error: %v", err)
	}
	first := inbound[0]
	second := inbound[1]
	firstOutbound := OutboundData{FileName: "出库一.xls", OutboundDate: "2026-07-14", DeclaredTotal: 1, Records: []OutboundRecord{{UPC: first.UPC, TrackingNumber: first.OrderNumber, Identifier: first.Identifier}}}
	secondOutbound := OutboundData{FileName: "出库二.xls", OutboundDate: "2026-07-15", DeclaredTotal: 1, Records: []OutboundRecord{{UPC: second.UPC, TrackingNumber: second.OrderNumber, Identifier: second.Identifier}}}
	report := reconcileInventories(inbound, []OutboundData{firstOutbound, secondOutbound})
	if !report.Reconciliation.Valid || len(report.Outbounds) != 2 || report.Reconciliation.RemainingTotal != 869 {
		t.Fatalf("unexpected multi-outbound reconciliation: %+v", report.Reconciliation)
	}
	path := filepath.Join(t.TempDir(), "multi-outbound.xlsx")
	if err := writeInventoryReportFile(report, "测试人", path); err != nil {
		t.Fatalf("writeInventoryReportFile returned an error: %v", err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("generated multi-outbound workbook could not be opened: %v", err)
	}
	defer func() { _ = book.Close() }()
	expected := "入库表,入库-透视,入库-概览,出库表,出库-透视,出库-概览,出库表-2,出库-透视-2,出库-概览-2,留仓表,留仓-透视,留仓-概览"
	if sheets := book.GetSheetList(); strings.Join(sheets, ",") != expected {
		t.Fatalf("unexpected multi-outbound sheet list: %v", sheets)
	}
}
