export namespace main {
	
	export class PivotConfig {
	    rowFields: string[];
	    columnField: string;
	    valueField: string;
	    aggregation: string;
	    reportPerson: string;
	    matchMode: string;
	
	    static createFrom(source: any = {}) {
	        return new PivotConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowFields = source["rowFields"];
	        this.columnField = source["columnField"];
	        this.valueField = source["valueField"];
	        this.aggregation = source["aggregation"];
	        this.reportPerson = source["reportPerson"];
	        this.matchMode = source["matchMode"];
	    }
	}
	export class ImportResult {
	    fileName: string;
	    headers: string[];
	    numericFields: string[];
	    rowCount: number;
	    suggestedConfig: PivotConfig;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.headers = source["headers"];
	        this.numericFields = source["numericFields"];
	        this.rowCount = source["rowCount"];
	        this.suggestedConfig = this.convertValues(source["suggestedConfig"], PivotConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OutboundImportResult {
	    id: string;
	    fileName: string;
	    declaredTotal: number;
	    detailTotal: number;
	    boxCount: number;
	
	    static createFrom(source: any = {}) {
	        return new OutboundImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.declaredTotal = source["declaredTotal"];
	        this.detailTotal = source["detailTotal"];
	        this.boxCount = source["boxCount"];
	    }
	}
	
	export class PivotRow {
	    labels: string[];
	    values: number[];
	    total: number;
	    isTotal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PivotRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.labels = source["labels"];
	        this.values = source["values"];
	        this.total = source["total"];
	        this.isTotal = source["isTotal"];
	    }
	}
	export class PivotResult {
	    rowHeaders: string[];
	    columnHeaders: string[];
	    metricLabel: string;
	    rows: PivotRow[];
	    totalRows: number;
	
	    static createFrom(source: any = {}) {
	        return new PivotResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rowHeaders = source["rowHeaders"];
	        this.columnHeaders = source["columnHeaders"];
	        this.metricLabel = source["metricLabel"];
	        this.rows = this.convertValues(source["rows"], PivotRow);
	        this.totalRows = source["totalRows"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class UnmatchedRecord {
	    fileName: string;
	    boxNumber: number;
	    trackingNumber: string;
	    upc: string;
	    identifier: string;
	    productName: string;
	
	    static createFrom(source: any = {}) {
	        return new UnmatchedRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.boxNumber = source["boxNumber"];
	        this.trackingNumber = source["trackingNumber"];
	        this.upc = source["upc"];
	        this.identifier = source["identifier"];
	        this.productName = source["productName"];
	    }
	}
	export class ReconciliationResult {
	    hasOutbound: boolean;
	    valid: boolean;
	    inboundTotal: number;
	    outboundDeclaredTotal: number;
	    outboundDetailTotal: number;
	    matchedTotal: number;
	    remainingTotal: number;
	    unmatched: UnmatchedRecord[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ReconciliationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasOutbound = source["hasOutbound"];
	        this.valid = source["valid"];
	        this.inboundTotal = source["inboundTotal"];
	        this.outboundDeclaredTotal = source["outboundDeclaredTotal"];
	        this.outboundDetailTotal = source["outboundDetailTotal"];
	        this.matchedTotal = source["matchedTotal"];
	        this.remainingTotal = source["remainingTotal"];
	        this.unmatched = this.convertValues(source["unmatched"], UnmatchedRecord);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveResult {
	    path: string;
	    fileName: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	    }
	}

}

