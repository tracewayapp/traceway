import type { AttemptEvent, AttemptStatus } from '$lib/types/agent';

export const TERMINAL_STATUSES: AttemptStatus[] = [
	'analyzed',
	'merged',
	'closed',
	'failed',
	'cancelled',
	'timed_out'
];

export function isTerminal(status: string): boolean {
	return (TERMINAL_STATUSES as string[]).includes(status);
}

export type PillTone = 'success' | 'danger' | 'warning' | 'info' | 'neutral' | 'violet';

export function statusTone(status: string): PillTone {
	switch (status) {
		case 'merged':
			return 'success';
		case 'awaiting_review':
			return 'violet';
		case 'needs_input':
		case 'pending_approval':
			return 'warning';
		case 'failed':
		case 'timed_out':
			return 'danger';
		case 'analyzed':
		case 'closed':
		case 'cancelled':
			return 'neutral';
		default:
			return 'info';
	}
}

export function statusLabel(status: string): string {
	const words = status.replaceAll('_', ' ');
	return words.charAt(0).toUpperCase() + words.slice(1);
}

// Merges a polled page of events into the ones already shown: the poll asks
// for everything after the last seq, so a duplicate only arrives on a retry
// or a reload, and the result stays sorted by seq either way.
export function mergeEvents(existing: AttemptEvent[], incoming: AttemptEvent[]): AttemptEvent[] {
	if (incoming.length === 0) return existing;
	const bySeq = new Map<number, AttemptEvent>();
	for (const event of existing) bySeq.set(event.seq, event);
	for (const event of incoming) bySeq.set(event.seq, event);
	return [...bySeq.values()].sort((a, b) => a.seq - b.seq);
}

export function latestSeq(events: AttemptEvent[]): number {
	return events.length === 0 ? 0 : events[events.length - 1].seq;
}

export function formatCost(usd: number): string {
	if (!usd) return '$0.00';
	return usd < 0.01 ? `$${usd.toFixed(4)}` : `$${usd.toFixed(2)}`;
}

export function attemptTitle(subjectKind: string, subjectRef: string): string {
	if (subjectKind === 'traceway_exception') return `Issue ${subjectRef}`;
	return `${subjectKind} ${subjectRef}`;
}
