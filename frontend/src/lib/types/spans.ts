// traceId, spanId and parentSpanId are the ids the span arrived with, as lowercase hex: 32 characters for a trace, 16
// for an OTel span, 32 for a span of the native protocol.
export type Span = {
	projectId: string;
	traceId: string;
	spanId: string;
	parentSpanId?: string;
	name: string;
	startTime: string; // ISO datetime
	duration: number; // nanoseconds
	recordedAt: string;
	spanKind?: number;
	statusCode?: number;
	serviceName?: string;
	scopeName?: string;
	attributes?: Record<string, string> | null;
	attributesOmitted?: boolean;
	dbStatement?: string;
};

// The ids every endpoint, task and AI trace carries next to its own row id. linkedTraceId is another trace the row
// belongs with, such as the browser trace that started the request.
export type TraceIdentity = {
	traceId: string;
	spanId: string;
	parentSpanId?: string;
	linkedTraceId?: string;
};

export type SpanAttributes = {
	attributes: Record<string, string>;
	attributesOmitted?: boolean;
};

export type SpanAttributeLoader = (span: Span) => Promise<SpanAttributes>;

export type SpanGraphStatus = {
	state: 'complete' | 'partial' | 'unavailable';
	reasons?: string[];
	omittedAttributes?: number;
};

export type TraceDetail = TraceIdentity & {
	id: string;
	projectId: string;
	endpoint: string;
	duration: number;
	recordedAt: string;
	statusCode: number;
	bodySize: number;
	clientIP: string;
	attributes: Record<string, string> | null;
	appVersion: string;
	serverName: string;
};

export type ExceptionInfo = {
	exceptionHash: string;
	stackTrace: string;
	recordedAt: string;
};

export type MessageInfo = {
	id: string;
	exceptionHash: string;
	stackTrace: string;
	recordedAt: string;
	attributes?: Record<string, string>;
};

export type TraceDetailResponse = {
	spanGraphStatus?: SpanGraphStatus;
	endpoint: TraceDetail;
	spans: Span[];
	hasSpans: boolean;
	exception?: ExceptionInfo;
	messages: MessageInfo[];
};
