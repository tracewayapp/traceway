import { api } from '$lib/api';
import type { SessionActionEvent, SessionLogEvent } from '$lib/types/exceptions';

export type SessionBreadcrumbs = {
	logs: SessionLogEvent[];
	actions: SessionActionEvent[];
	truncated: boolean;
};

// The widest window GET /api/sessions/:id/timeline accepts.
export const MAX_TIMELINE_WINDOW_MS = 3 * 60 * 60 * 1000;

export function timelineWindow(fromMs: number, toMs: number): { from: string; to: string } {
	const to = Math.min(Math.max(toMs, fromMs), fromMs + MAX_TIMELINE_WINDOW_MS);
	return { from: new Date(fromMs).toISOString(), to: new Date(to).toISOString() };
}

/**
 * The console lines, requests, navigation and app events of a session, read from its telemetry. Null when the
 * read fails: the breadcrumbs only add context, so the page shows what it has instead of an error.
 */
export async function loadSessionTimeline(
	sessionId: string,
	fromMs: number,
	toMs: number,
	projectId?: string
): Promise<SessionBreadcrumbs | null> {
	const { from, to } = timelineWindow(fromMs, toMs);
	try {
		const timeline = await api.get(
			`/sessions/${sessionId}/timeline?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
			{ projectId }
		);
		return {
			logs: timeline?.logs ?? [],
			actions: timeline?.actions ?? [],
			truncated: !!timeline?.truncated
		};
	} catch {
		return null;
	}
}

/** Older replay files and OTLP may both contribute context during an SDK rollout. */
export function pickBreadcrumbs(
	timeline: SessionBreadcrumbs | null,
	recording?: { logs?: SessionLogEvent[]; actions?: SessionActionEvent[] } | null
): SessionBreadcrumbs {
	function merge<T extends { timestamp: string }>(legacy: T[] = [], current: T[] = []): T[] {
		const rows = new Map<string, T>();
		for (const row of [...legacy, ...current]) rows.set(JSON.stringify(row), row);
		return [...rows.values()].sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp));
	}
	return {
		logs: merge(recording?.logs, timeline?.logs),
		actions: merge(recording?.actions, timeline?.actions),
		truncated: timeline?.truncated ?? false
	};
}
