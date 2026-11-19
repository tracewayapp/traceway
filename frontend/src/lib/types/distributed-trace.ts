export type DistributedTraceNode = {
	spanGraphStatus?: SpanGraphStatus;
	projectId: string;
	projectName: string;
	traceType: 'endpoint' | 'task' | 'ai_trace' | 'exception';
	traceId: string;
	spanId: string;
	parentEntitySpanId?: string;
	endpoint?: {
		id: string;
		parentSpanId?: string;
		endpoint: string;
		duration: number;
		statusCode: number;
		recordedAt: string;
	};
	task?: {
		id: string;
		parentSpanId?: string;
		taskName: string;
		duration: number;
		recordedAt: string;
	};
	aiTrace?: {
		id: string;
		parentSpanId?: string;
		traceName: string;
		model: string;
		provider: string;
		duration: number;
		totalTokens: number;
		totalCost: number;
		recordedAt: string;
	};
	spans: Span[];
	exception?: {
		exceptionHash: string;
		stackTrace: string;
		recordedAt: string;
	} | null;
};

export type DistributedTraceResponse = {
	traceId: string;
	nodes: DistributedTraceNode[];
};
import type { Span, SpanGraphStatus } from '$lib/types/spans';
