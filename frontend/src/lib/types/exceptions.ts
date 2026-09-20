export type ExceptionGroup = {
	exceptionHash: string;
	stackTrace: string;
	lastSeen: string;
	firstSeen: string;
	count: number;
};

export type ExceptionOccurrence = {
	id: string;
	// The trace and span the exception was recorded on, as lowercase hex. Empty when it arrived without a trace.
	traceId: string;
	spanId: string;
	traceType: 'endpoint' | 'task' | 'ai_trace' | '';
	linkedTraceId?: string;
	exceptionHash: string;
	stackTrace: string;
	recordedAt: string;
	attributes: Record<string, string> | null;
	appVersion: string;
	serverName: string;
	isMessage: boolean;
	endpoint: string;
};

// The endpoint, task or AI trace an exception happened in, as the backend resolves it.
export type RelatedEntity = {
	traceType: 'endpoint' | 'task' | 'ai_trace';
	id: string;
	name: string;
	statusCode: number;
	duration: number;
	recordedAt: string;
	traceId: string;
};

export type LinkedTrace = {
	id: string;
	endpoint: string;
	duration: number;
	statusCode: number;
	recordedAt: string;
	traceType: 'endpoint' | 'task' | 'ai_trace';
	traceId: string;
};

export function linkedTraceFrom(related: RelatedEntity | null | undefined): LinkedTrace | null {
	if (!related) return null;
	return {
		id: related.id,
		endpoint: related.name,
		duration: related.duration,
		statusCode: related.statusCode,
		recordedAt: related.recordedAt,
		traceType: related.traceType,
		traceId: related.traceId
	};
}

// Session recording shape returned by the backend's exception detail endpoints.
// Mirrors the wire format produced by the Flutter and JS SDKs.

export type SessionLogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface SessionLogEvent {
	type: 'log';
	timestamp: string;
	level: SessionLogLevel;
	message: string;
}

export interface SessionNetworkEvent {
	type: 'network';
	timestamp: string;
	method: string;
	url: string;
	durationMs: number;
	statusCode?: number;
	requestBytes?: number;
	responseBytes?: number;
	error?: string;
}

export interface SessionNavigationEvent {
	type: 'navigation';
	timestamp: string;
	/** push | pop | replace | remove */
	action: string;
	from?: string;
	to?: string;
}

export interface SessionCustomEvent {
	type: 'custom';
	timestamp: string;
	category: string;
	name: string;
	data?: Record<string, unknown>;
}

export type SessionActionEvent = SessionNetworkEvent | SessionNavigationEvent | SessionCustomEvent;

export interface FlutterVideoEvent {
	type: 'flutter_video';
	data: string;
	format: string;
	fps: number;
	durationSeconds: number;
}

export type SessionReplayEvent = (eventWithTime & { data?: { href?: string } }) | FlutterVideoEvent;

export interface SessionRecording {
	/** rrweb events (web SDKs) or MP4 chunk descriptors (Flutter SDK). */
	events?: SessionReplayEvent[];
	/** Console output snapshotted at exception capture (last ~10s, ≤200). */
	logs?: SessionLogEvent[];
	/** Network / navigation / custom actions snapshotted with the exception. */
	actions?: SessionActionEvent[];
	/** ISO 8601 timestamp of the first frame / first event. */
	startedAt?: string;
	/** ISO 8601 timestamp of the last frame / last event. */
	endedAt?: string;
}
import type { eventWithTime } from '@rrweb/types';
