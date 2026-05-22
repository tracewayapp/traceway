export type Span = {
	id: string;
	traceId: string;
	projectId: string;
	name: string;
	startTime: string; // ISO datetime
	duration: number; // nanoseconds
	recordedAt: string;
	parentSpanId?: string;
	linkedAiTraceId?: string;
	linkedAiTraceName?: string;
};

export type TraceDetail = {
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
	distributedTraceId?: string;
	spanId?: string;
	traceId?: string;
	parentSpanId?: string;
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

export type ParentRef = {
	kind: 'endpoint' | 'task' | 'ai_trace';
	id: string;
	name: string;
	traceId: string;
};

// ChildEntity is an endpoint/task/ai_trace nested inside the current row's
// owned-span subtree (transitive). The detail controllers compute these and
// the waterfall renders them as click-targets so the user can navigate down.
export type ChildEntity = {
	kind: 'endpoint' | 'task' | 'ai_trace';
	id: string;
	name: string;
	parentSpanId: string;
	traceId: string;
	recordedAt: string;
	duration: number; // nanoseconds
};

export type TraceDetailResponse = {
	endpoint: TraceDetail;
	spans: Span[];
	hasSpans: boolean;
	exception?: ExceptionInfo;
	messages: MessageInfo[];
	parentRef?: ParentRef;
	childEntities: ChildEntity[];
};
