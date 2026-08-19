export type DistributedTraceNode = {
	projectId: string;
	projectName: string;
	traceType: 'endpoint' | 'task' | 'ai_trace' | 'exception';
	endpoint?: {
		id: string;
		endpoint: string;
		duration: number;
		statusCode: number;
		recordedAt: string;
	};
	task?: {
		id: string;
		taskName: string;
		duration: number;
		recordedAt: string;
	};
	aiTrace?: {
		id: string;
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
	distributedTraceId: string;
	nodes: DistributedTraceNode[];
};
import type { Span } from '$lib/types/spans';
