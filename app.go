package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App owns the imported dataset for the lifetime of the desktop session.
type App struct {
	ctx       context.Context
	mu        sync.RWMutex
	dataset   *Dataset
	outbounds []OutboundData
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ResetSession clears all imported data for the current application session.
func (a *App) ResetSession() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dataset = nil
	a.outbounds = nil
}

// SelectSourceFile opens the system file picker and loads the selected Excel file.
func (a *App) SelectSourceFile() (ImportResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Excel 数据源",
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel 文件", Pattern: "*.xls;*.xlsx"},
		},
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("选择文件失败: %w", err)
	}
	if path == "" {
		return ImportResult{}, fmt.Errorf("未选择文件")
	}

	dataset, err := loadDataset(path)
	if err != nil {
		return ImportResult{}, err
	}
	a.mu.Lock()
	a.dataset = &dataset
	a.mu.Unlock()
	return ImportResult{
		FileName:        dataset.FileName,
		Headers:         dataset.Headers,
		NumericFields:   numericFields(dataset),
		RowCount:        len(dataset.Rows),
		SuggestedConfig: suggestedPivotConfig(dataset),
	}, nil
}

// PreviewPivot calculates the selected pivot without writing a file.
func (a *App) PreviewPivot(config PivotConfig) (PivotResult, error) {
	dataset, err := a.currentDataset()
	if err != nil {
		return PivotResult{}, err
	}
	return buildPivot(dataset, config)
}

// SelectOutboundFile loads the fixed-format system export used for shipment and stock reconciliation.
func (a *App) SelectOutboundFile() (OutboundImportResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择出库 Excel 文件",
		Filters: []runtime.FileFilter{{DisplayName: "Excel 文件", Pattern: "*.xls;*.xlsx"}},
	})
	if err != nil {
		return OutboundImportResult{}, fmt.Errorf("选择出库文件失败: %w", err)
	}
	if path == "" {
		return OutboundImportResult{}, fmt.Errorf("未选择文件")
	}
	outbound, err := parseOutboundFile(path)
	if err != nil {
		return OutboundImportResult{}, err
	}
	a.mu.Lock()
	a.outbounds = append(a.outbounds, outbound)
	a.mu.Unlock()
	return OutboundImportResult{
		ID: outbound.Path, FileName: outbound.FileName, DeclaredTotal: outbound.DeclaredTotal,
		DetailTotal: len(outbound.Records), BoxCount: len(outbound.Boxes),
	}, nil
}

// RemoveOutboundFile removes a previously imported outbound workbook from the current session.
func (a *App) RemoveOutboundFile(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for index, outbound := range a.outbounds {
		if outbound.Path == id {
			a.outbounds = append(a.outbounds[:index], a.outbounds[index+1:]...)
			return nil
		}
	}
	return fmt.Errorf("未找到需要移除的出库文件")
}

// GetReconciliation returns validation and unmatched shipment records for the active imports.
func (a *App) GetReconciliation(matchMode string) (ReconciliationResult, error) {
	dataset, outbounds, err := a.currentSources()
	if err != nil {
		return ReconciliationResult{}, err
	}
	if len(outbounds) == 0 {
		return ReconciliationResult{HasOutbound: false, Valid: true, InboundTotal: quantityOfInboundMust(dataset)}, nil
	}
	inbound, err := inboundRecords(dataset)
	if err != nil {
		return ReconciliationResult{}, err
	}
	result := reconcileInventoriesWithMatchMode(inbound, outbounds, matchMode).Reconciliation
	sortUnmatched(result.Unmatched)
	return result, nil
}

// SaveReport writes the inbound-only or reconciled inventory workbook to the desktop.
func (a *App) SaveReport(config PivotConfig, allowInvalid bool) (SaveResult, error) {
	if strings.TrimSpace(config.ReportPerson) == "" {
		return SaveResult{}, fmt.Errorf("请填写报表人名")
	}
	dataset, outbounds, err := a.currentSources()
	if err != nil {
		return SaveResult{}, err
	}
	inbound, err := inboundRecords(dataset)
	if err != nil {
		return SaveResult{}, err
	}
	var report InventoryReportData
	if len(outbounds) == 0 {
		report = InventoryReportData{Inbound: inbound}
	} else {
		report = reconcileInventoriesWithMatchMode(inbound, outbounds, config.MatchMode)
		if !report.Reconciliation.Valid && !allowInvalid {
			return SaveResult{}, fmt.Errorf("校验未通过，无法生成九个 Sheet 报表：%s", strings.Join(report.Reconciliation.Errors, "；"))
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Excel 报表",
		DefaultFilename: fmt.Sprintf("Apple_出入库报表_%s.xlsx", time.Now().Format("20060102")),
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel 文件", Pattern: "*.xlsx"},
		},
	})
	if err != nil {
		return SaveResult{}, fmt.Errorf("选择保存位置失败: %w", err)
	}
	if path == "" {
		return SaveResult{}, nil
	}
	if filepath.Ext(path) == "" {
		path += ".xlsx"
	}
	return writeInventoryReport(report, config.ReportPerson, path)
}

func (a *App) currentDataset() (Dataset, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dataset == nil {
		return Dataset{}, fmt.Errorf("请先导入 Excel 数据文件")
	}
	return *a.dataset, nil
}

func (a *App) currentSources() (Dataset, []OutboundData, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.dataset == nil {
		return Dataset{}, nil, fmt.Errorf("请先导入入库 Excel 数据文件")
	}
	return *a.dataset, append([]OutboundData(nil), a.outbounds...), nil
}

func quantityOfInboundMust(dataset Dataset) int {
	records, err := inboundRecords(dataset)
	if err != nil {
		return 0
	}
	return quantityOfInbound(records)
}
