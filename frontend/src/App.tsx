import {useMemo, useState} from 'react';
import {
    ArrowUpRight,
    AlertTriangle,
    Box,
    Check,
    CheckCircle2,
    ChevronDown,
    Database,
    FileSpreadsheet,
    FolderOpen,
    Layers3,
    LoaderCircle,
    Save,
    RotateCcw,
    SlidersHorizontal,
    Table2,
    Truck,
    XCircle,
} from 'lucide-react';
import {GetReconciliation, PreviewPivot, RemoveOutboundFile, ResetSession, SaveReport, SelectOutboundFile, SelectSourceFile} from '../wailsjs/go/main/App';
import './App.css';

type Aggregation = 'count' | 'sum';
type MatchMode = 'orderNumber' | 'imei';

type PivotConfig = {
    rowFields: string[];
    columnField: string;
    valueField: string;
    aggregation: Aggregation;
    reportPerson: string;
    matchMode: MatchMode;
};

type ImportResult = {
    fileName: string;
    headers: string[];
    numericFields: string[];
    rowCount: number;
    suggestedConfig: PivotConfig;
};

type PivotRow = {
    labels: string[];
    values: number[];
    total: number;
    isTotal: boolean;
};

type PivotResult = {
    rowHeaders: string[];
    columnHeaders: string[];
    metricLabel: string;
    rows: PivotRow[];
    totalRows: number;
};

type SaveResult = {
    path: string;
    fileName: string;
};

type OutboundImportResult = {
    id: string;
    fileName: string;
    declaredTotal: number;
    detailTotal: number;
    boxCount: number;
};

type UnmatchedRecord = {
    fileName: string;
    boxNumber: number;
    trackingNumber: string;
    upc: string;
    identifier: string;
    productName: string;
};

type ReconciliationResult = {
    hasOutbound: boolean;
    valid: boolean;
    inboundTotal: number;
    outboundDeclaredTotal: number;
    outboundDetailTotal: number;
    matchedTotal: number;
    remainingTotal: number;
    unmatched: UnmatchedRecord[];
    errors: string[];
};

const emptyConfig: PivotConfig = {
    rowFields: [],
    columnField: '',
    valueField: '',
    aggregation: 'count',
    reportPerson: '',
    matchMode: 'orderNumber',
};

function errorText(error: unknown) {
    return error instanceof Error ? error.message : String(error);
}

function formatNumber(value: number) {
    return new Intl.NumberFormat('zh-CN', {maximumFractionDigits: 0}).format(value);
}

function App() {
    const [source, setSource] = useState<ImportResult | null>(null);
    const [config, setConfig] = useState<PivotConfig>(emptyConfig);
    const [preview, setPreview] = useState<PivotResult | null>(null);
    const [outbounds, setOutbounds] = useState<OutboundImportResult[]>([]);
    const [reconciliation, setReconciliation] = useState<ReconciliationResult | null>(null);
    const [busy, setBusy] = useState<'import' | 'outbound' | 'preview' | 'save' | 'reset' | 'match' | null>(null);
    const [error, setError] = useState('');
    const [savedReport, setSavedReport] = useState<SaveResult | null>(null);
    const [showInvalidConfirm, setShowInvalidConfirm] = useState(false);

    const fields = source?.headers ?? [];
    const selectedRowsText = useMemo(() => config.rowFields.join(' / '), [config.rowFields]);
    const dataRows = preview?.rows.filter((row) => !row.isTotal) ?? [];
    const outboundDetailTotal = outbounds.reduce((total, outbound) => total + outbound.detailTotal, 0);
    const canSave = Boolean(preview && config.reportPerson.trim());
    const matchFieldLabel = config.matchMode === 'imei' ? 'IMEI' : '单号';

    const updateConfig = (changes: Partial<PivotConfig>) => {
        setConfig((current) => ({...current, ...changes}));
        setPreview(null);
        setSavedReport(null);
    };

    const updateReportPerson = (reportPerson: string) => {
        setConfig((current) => ({...current, reportPerson}));
        setSavedReport(null);
    };

    const toggleRowField = (field: string) => {
        setConfig((current) => {
            const rowFields = current.rowFields.includes(field)
                ? current.rowFields.filter((value) => value !== field)
                : [...current.rowFields, field];
            return {...current, rowFields};
        });
        setPreview(null);
        setSavedReport(null);
    };

    const refreshReconciliation = async (matchMode: MatchMode = config.matchMode) => {
        try {
            setReconciliation(await GetReconciliation(matchMode) as ReconciliationResult);
        } catch (cause) {
            setError(errorText(cause));
        }
    };

    const selectMatchMode = async (matchMode: MatchMode) => {
        if (matchMode === config.matchMode || busy !== null) {
            return;
        }
        setBusy('match');
        setError('');
        setSavedReport(null);
        setShowInvalidConfirm(false);
        setConfig((current) => ({...current, matchMode}));
        try {
            if (outbounds.length > 0) {
                setReconciliation(await GetReconciliation(matchMode) as ReconciliationResult);
            }
        } catch (cause) {
            setError(errorText(cause));
        } finally {
            setBusy(null);
        }
    };

    const previewPivot = async (nextConfig: PivotConfig = config) => {
        if (nextConfig.rowFields.length === 0) {
            setError('请选择至少一个行字段。');
            return;
        }
        setBusy('preview');
        setError('');
        setSavedReport(null);
        try {
            setPreview(await PreviewPivot(nextConfig) as PivotResult);
        } catch (cause) {
            setError(errorText(cause));
        } finally {
            setBusy(null);
        }
    };

    const importSource = async () => {
        setBusy('import');
        setError('');
        setSavedReport(null);
        try {
            const imported = await SelectSourceFile() as ImportResult;
            setSource(imported);
            const nextConfig = {...imported.suggestedConfig, reportPerson: config.reportPerson, matchMode: config.matchMode};
            setConfig(nextConfig);
            setBusy('preview');
            setPreview(await PreviewPivot(nextConfig) as PivotResult);
            if (outbounds.length > 0) {
                await refreshReconciliation(nextConfig.matchMode);
            }
        } catch (cause) {
            const message = errorText(cause);
            if (!message.includes('未选择文件')) {
                setError(message);
            }
        } finally {
            setBusy(null);
        }
    };

    const importOutbound = async () => {
        if (!source) {
            setError('请先导入入库表。');
            return;
        }
        setBusy('outbound');
        setError('');
        setSavedReport(null);
        try {
            const imported = await SelectOutboundFile() as OutboundImportResult;
            setOutbounds((current) => [...current, imported]);
            await refreshReconciliation();
        } catch (cause) {
            const message = errorText(cause);
            if (!message.includes('未选择文件')) {
                setError(message);
            }
        } finally {
            setBusy(null);
        }
    };

    const removeOutbound = async (id: string) => {
        setBusy('outbound');
        setError('');
        try {
            await RemoveOutboundFile(id);
            setOutbounds((current) => current.filter((item) => item.id !== id));
            await refreshReconciliation();
        } catch (cause) {
            setError(errorText(cause));
        } finally {
            setBusy(null);
        }
    };

    const resetSession = async () => {
        setBusy('reset');
        setError('');
        try {
            await ResetSession();
            setSource(null);
            setConfig(emptyConfig);
            setPreview(null);
            setOutbounds([]);
            setReconciliation(null);
            setSavedReport(null);
            setShowInvalidConfirm(false);
        } catch (cause) {
            setError(errorText(cause));
        } finally {
            setBusy(null);
        }
    };

    const saveReport = async (allowInvalid = false) => {
        if (!preview) {
            return;
        }
        setBusy('save');
        setError('');
        setSavedReport(null);
        try {
            const result = await SaveReport(config, allowInvalid) as SaveResult;
            if (result.path) {
                setSavedReport(result);
            }
        } catch (cause) {
            setError(errorText(cause));
        } finally {
            setBusy(null);
        }
    };

    const requestSave = () => {
        if (outbounds.length > 0 && reconciliation && !reconciliation.valid) {
            setShowInvalidConfirm(true);
            return;
        }
        void saveReport();
    };

    return (
        <main className="workspace">
            <header className="topbar">
                <div className="brand-lockup">
                    <div className="brand-mark" aria-hidden="true">S</div>
                    <div>
                        <div className="brand-name">Sail 工作助手</div>
                    </div>
                </div>
                <div className="topbar-meta">
                    <div className="match-mode-control" role="radiogroup" aria-label="出入库校验方式">
                        <span className="match-mode-label">校验</span>
                        <label className={`match-mode-option ${config.matchMode === 'orderNumber' ? 'active' : ''}`}>
                            <input type="radio" name="match-mode" value="orderNumber" checked={config.matchMode === 'orderNumber'} onChange={() => void selectMatchMode('orderNumber')} disabled={busy !== null}/>
                            <span>单号</span>
                        </label>
                        <label className={`match-mode-option ${config.matchMode === 'imei' ? 'active' : ''}`}>
                            <input type="radio" name="match-mode" value="imei" checked={config.matchMode === 'imei'} onChange={() => void selectMatchMode('imei')} disabled={busy !== null}/>
                            <span>IMEI</span>
                        </label>
                    </div>
                    {source ? (
                        <span className="source-status"><span className="status-dot"/> 入库 {source.rowCount.toLocaleString()} 条{outbounds.length > 0 ? ` · ${outbounds.length} 份出库表 / ${outboundDetailTotal} 条` : ''}</span>
                    ) : (
                        <span className="source-status muted"><span className="status-dot"/> 等待数据源</span>
                    )}
                    <button className="button button-secondary reset-button" onClick={() => void resetSession()} disabled={busy !== null}>
                        {busy === 'reset' ? <LoaderCircle className="spin" size={16}/> : <RotateCcw size={16}/>} 恢复
                    </button>
                </div>
            </header>

            <section className="workbench">
                <aside className="configuration-panel" aria-label="透视配置">
                    <div className="panel-heading">
                        <div className="heading-icon teal"><SlidersHorizontal size={18}/></div>
                        <div>
                            <p className="eyebrow">配置</p>
                            <h1>数据透视设置</h1>
                        </div>
                    </div>

                    {!source ? (
                        <div className="empty-config">
                            <div className="empty-icon"><FileSpreadsheet size={25}/></div>
                            <strong>暂未导入数据</strong>
                            <span>导入 Excel 后配置透视字段</span>
                            <button className="button button-primary" onClick={importSource} disabled={busy !== null}>
                                <FolderOpen size={17}/> 选择 Excel 文件
                            </button>
                        </div>
                    ) : (
                        <div className="settings-stack">
                            <section className="source-section">
                                <div className="source-row">
                                    <span><FileSpreadsheet size={16}/> 入库表</span>
                                    <strong>{source.fileName}</strong>
                                </div>
                                <button className="button button-secondary full-width" onClick={importSource} disabled={busy !== null}>
                                    <FolderOpen size={16}/> 更换入库表
                                </button>
                            </section>
                            <section className="source-section">
                                <div className="source-row">
                                    <span><Truck size={16}/> 出库表</span>
                                    <strong>{outbounds.length > 0 ? `已导入 ${outbounds.length} 份` : '未导入'}</strong>
                                </div>
                                {outbounds.length > 0 && (
                                    <div className="outbound-file-list">
                                        {outbounds.map((item) => (
                                            <div className="outbound-file" key={item.id}>
                                                <span>{item.fileName} · {item.detailTotal} 条</span>
                                                <button className="remove-outbound" title="移除出库表" onClick={() => void removeOutbound(item.id)} disabled={busy !== null}><XCircle size={15}/></button>
                                            </div>
                                        ))}
                                    </div>
                                )}
                                <button className="button button-secondary full-width" onClick={importOutbound} disabled={busy !== null}>
                                    {busy === 'outbound' ? <LoaderCircle className="spin" size={16}/> : <FolderOpen size={16}/>}
                                    添加出库表（可选）
                                </button>
                            </section>
                            <section className="setting-section">
                                <label className="setting-label" htmlFor="report-person">报表人名 <span className="required">*</span></label>
                                <input
                                    id="report-person"
                                    className="report-person-input"
                                    value={config.reportPerson}
                                    onChange={(event) => updateReportPerson(event.target.value)}
                                    placeholder="输入人名"
                                    autoComplete="off"
                                />
                            </section>
                            <section className="setting-section">
                                <div className="setting-title-row">
                                    <label>行字段 <span className="required">*</span></label>
                                    <span className="selection-count">{config.rowFields.length} 已选</span>
                                </div>
                                <div className="field-list">
                                    {fields.map((field) => {
                                        const selected = config.rowFields.includes(field);
                                        return (
                                            <label className={`field-toggle ${selected ? 'selected' : ''}`} key={field}>
                                                <input type="checkbox" checked={selected} onChange={() => toggleRowField(field)}/>
                                                <span className="custom-checkbox">{selected && <Check size={13}/>}</span>
                                                <span>{field}</span>
                                            </label>
                                        );
                                    })}
                                </div>
                            </section>

                            <section className="setting-section">
                                <label className="setting-label" htmlFor="column-field">列字段</label>
                                <div className="select-wrap">
                                    <select id="column-field" value={config.columnField} onChange={(event) => updateConfig({columnField: event.target.value})}>
                                        <option value="">不按列拆分</option>
                                        {fields.map((field) => <option value={field} key={field}>{field}</option>)}
                                    </select>
                                    <ChevronDown size={15}/>
                                </div>
                            </section>

                            <section className="setting-section">
                                <label className="setting-label">汇总方式</label>
                                <div className="segmented-control" role="group" aria-label="汇总方式">
                                    <button className={config.aggregation === 'sum' ? 'active' : ''} onClick={() => updateConfig({aggregation: 'sum', valueField: config.valueField || '数量'})}>求和</button>
                                    <button className={config.aggregation === 'count' ? 'active' : ''} onClick={() => updateConfig({aggregation: 'count'})}>计数</button>
                                </div>
                            </section>

                            {config.aggregation === 'sum' && (
                                <section className="setting-section">
                                    <label className="setting-label">数值字段</label>
                                    <div className="readonly-field">数量</div>
                                </section>
                            )}

                            <button className="button button-primary full-width" onClick={() => previewPivot()} disabled={busy !== null || config.rowFields.length === 0}>
                                {busy === 'preview' ? <LoaderCircle className="spin" size={17}/> : <Table2 size={17}/>} 
                                生成透视预览
                            </button>
                        </div>
                    )}
                </aside>

                <section className="preview-panel" aria-label="透视数据预览">
                    <div className="preview-toolbar">
                        <div>
                            <p className="eyebrow">预览</p>
                            <div className="preview-title-line">
                                <h2>数据透视表</h2>
                                {preview && <span className="table-chip"><Layers3 size={13}/>{dataRows.length} 个分组</span>}
                            </div>
                        </div>
                        {source && (
                            <div className="file-identity">
                                <FileSpreadsheet size={17}/>
                                <span>{source.fileName}</span>
                            </div>
                        )}
                    </div>

                    {error && (
                        <div className="feedback feedback-error" role="alert">
                            <XCircle size={18}/><span>{error}</span>
                        </div>
                    )}

                    {savedReport && (
                        <div className="feedback feedback-success" role="status">
                            <CheckCircle2 size={18}/>
                            <span>已保存：{savedReport.fileName}</span>
                            <span className="saved-path">{savedReport.path}</span>
                        </div>
                    )}

                    {outbounds.length > 0 && reconciliation && (
                        <section className={`reconciliation ${reconciliation.valid ? 'reconciliation-valid' : 'reconciliation-invalid'}`}>
                            <div className="reconciliation-heading">
                                {reconciliation.valid ? <CheckCircle2 size={19}/> : <AlertTriangle size={19}/>}
                                <div>
                                    <strong>{reconciliation.valid ? '出入库校验通过' : '出入库校验未通过'}</strong>
                                    <span>入库、出库与留仓数量校验</span>
                                </div>
                            </div>
                            <div className="reconciliation-grid">
                                <div><span>入库总数</span><strong>{formatNumber(reconciliation.inboundTotal)}</strong></div>
                                <div><span>出库总数（出库文件）</span><strong>{formatNumber(reconciliation.outboundDeclaredTotal)}</strong></div>
                                <div><span>出库明细数（逐台读取）</span><strong>{formatNumber(reconciliation.outboundDetailTotal)}</strong></div>
                                <div><span>留仓总数</span><strong>{formatNumber(reconciliation.remainingTotal)}</strong></div>
                            </div>
                            {!reconciliation.valid && (
                                <>
                                    <div className="validation-errors">{reconciliation.errors.map((item) => <span key={item}>{item}</span>)}</div>
                                    {reconciliation.unmatched.length > 0 && (
                                        <div className="unmatched-block">
                                            <div className="unmatched-title"><Box size={15}/> 未匹配出库{matchFieldLabel} ({reconciliation.unmatched.length})</div>
                                            <div className="unmatched-table-wrap">
                                                <table className="unmatched-table">
                                                    <thead><tr><th>出库表</th><th>箱号</th><th>单号</th><th>UPC</th><th>IMEI</th></tr></thead>
                                                    <tbody>{reconciliation.unmatched.map((item) => <tr key={`${item.fileName}-${item.trackingNumber}`}><td>{item.fileName}</td><td>{item.boxNumber}</td><td>{item.trackingNumber}</td><td>{item.upc}</td><td>{item.identifier}</td></tr>)}</tbody>
                                                </table>
                                            </div>
                                        </div>
                                    )}
                                </>
                            )}
                        </section>
                    )}

                    {!source ? (
                        <div className="blank-preview">
                            <div className="blank-visual"><Database size={34}/></div>
                            <strong>数据透视预览</strong>
                            <span>导入数据源后将在这里显示结果</span>
                        </div>
                    ) : busy === 'preview' && !preview ? (
                        <div className="blank-preview"><LoaderCircle className="spin" size={30}/><span>正在计算透视数据</span></div>
                    ) : preview ? (
                        <>
                            <div className="preview-context">
                                <div><span>行字段</span><strong>{selectedRowsText || '未选择'}</strong></div>
                                <div><span>汇总</span><strong>{preview.metricLabel}</strong></div>
                                <div><span>原始数据</span><strong>{preview.totalRows.toLocaleString()} 条</strong></div>
                            </div>
                            <div className="table-scroll">
                                <table className="pivot-table">
                                    <thead>
                                    <tr>
                                        {preview.rowHeaders.map((header) => <th key={header}>{header}</th>)}
                                        {preview.columnHeaders.map((header) => <th className="numeric" key={header}>{header}</th>)}
                                    </tr>
                                    </thead>
                                    <tbody>
                                    {preview.rows.map((row, rowIndex) => (
                                        <tr className={row.isTotal ? 'total-row' : ''} key={`${row.labels.join('|')}-${rowIndex}`}>
                                            {row.labels.map((label, labelIndex) => <td key={`${label}-${labelIndex}`}>{label}</td>)}
                                            {row.values.map((value, valueIndex) => <td className="numeric" key={valueIndex}>{formatNumber(value)}</td>)}
                                        </tr>
                                    ))}
                                    </tbody>
                                </table>
                            </div>
                        </>
                    ) : null}
                </section>
            </section>

            <footer className="actionbar">
                <div className="actionbar-status">
                    {preview ? <><CheckCircle2 size={18}/> 透视数据已就绪，可保存为 Excel 报表</> : <><ArrowUpRight size={18}/> 导入数据并生成透视预览</>}
                </div>
                <button className="button button-save" onClick={requestSave} disabled={!canSave || busy !== null}>
                    {busy === 'save' ? <LoaderCircle className="spin" size={18}/> : <Save size={18}/>} 
                    选择位置并保存
                </button>
            </footer>

            {showInvalidConfirm && reconciliation && (
                <div className="confirm-backdrop" role="presentation">
                    <section className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="invalid-confirm-title">
                        <div className="confirm-icon"><AlertTriangle size={22}/></div>
                        <div>
                            <h2 id="invalid-confirm-title">校验未通过</h2>
                            <p>将导出已匹配的正常出库记录和对应留仓数据，忽略 {reconciliation.unmatched.length} 条未匹配{matchFieldLabel}。异常清单不会写入报表。</p>
                        </div>
                        <div className="confirm-actions">
                            <button className="button button-secondary" onClick={() => setShowInvalidConfirm(false)}>取消</button>
                            <button className="button button-danger" onClick={() => { setShowInvalidConfirm(false); void saveReport(true); }}>仍然保存</button>
                        </div>
                    </section>
                </div>
            )}
        </main>
    );
}

export default App;
