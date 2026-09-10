import { describe, expect, it } from 'vitest';
import { formatCost, isTerminal, latestSeq, mergeEvents, statusLabel, statusTone } from './agent';
import type { AttemptEvent } from '$lib/types/agent';

function event(seq: number, kind = 'progress'): AttemptEvent {
	return { id: seq, attemptId: 'a', seq, kind, payload: {}, createdAt: '' };
}

describe('mergeEvents', () => {
	it('appends new events after the cursor in seq order', () => {
		const merged = mergeEvents([event(1), event(2)], [event(3), event(4)]);
		expect(merged.map((e) => e.seq)).toEqual([1, 2, 3, 4]);
		expect(latestSeq(merged)).toBe(4);
	});

	it('drops duplicates a retried poll or a reload brings back', () => {
		const merged = mergeEvents([event(1), event(2)], [event(2, 'status'), event(3)]);
		expect(merged.map((e) => e.seq)).toEqual([1, 2, 3]);
		expect(merged[1].kind).toBe('status');
	});

	it('returns the existing list untouched when nothing arrived', () => {
		const existing = [event(1)];
		expect(mergeEvents(existing, [])).toBe(existing);
		expect(latestSeq([])).toBe(0);
	});
});

describe('status helpers', () => {
	it('knows which statuses stop polling', () => {
		for (const status of ['analyzed', 'merged', 'closed', 'failed', 'cancelled', 'timed_out']) {
			expect(isTerminal(status)).toBe(true);
		}
		for (const status of ['queued', 'running', 'needs_input', 'awaiting_review']) {
			expect(isTerminal(status)).toBe(false);
		}
	});

	it('maps statuses to pill tones', () => {
		expect(statusTone('merged')).toBe('success');
		expect(statusTone('needs_input')).toBe('warning');
		expect(statusTone('failed')).toBe('danger');
		expect(statusTone('running')).toBe('info');
		expect(statusLabel('needs_input')).toBe('Needs input');
		expect(statusLabel('queued')).toBe('Queued');
		expect(statusTone('awaiting_review')).toBe('violet');
		expect(statusTone('analyzed')).toBe('neutral');
	});

	it('formats cost with enough precision for cents and fractions of a cent', () => {
		expect(formatCost(0)).toBe('$0.00');
		expect(formatCost(0.0042)).toBe('$0.0042');
		expect(formatCost(1.2345)).toBe('$1.23');
	});
});
