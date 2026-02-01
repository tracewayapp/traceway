export type Span = {
	id: string;
	traceId: string;
	projectId: string;
	name: string;
	startTime: string; // ISO datetime
	duration: number; // nanoseconds
	recordedAt: string;
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
	endpoint: TraceDetail;
	spans: Span[];
	hasSpans: boolean;
	exception?: ExceptionInfo;
	messages: MessageInfo[];
};
