import { describe, expect, it, vi } from 'vitest';

vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));

import { sessionDurationLabel } from './session-duration';

const now = Date.parse('2026-09-26T17:00:00Z');
const minutesAgo = (m: number) => new Date(now - m * 60_000).toISOString();

describe('sessionDurationLabel', () => {
	it('shows the duration of an ended session', () => {
		expect(
			sessionDurationLabel(
				{ startedAt: minutesAgo(40), endedAt: minutesAgo(38), duration: 120e9 },
				now
			)
		).toBe('120.0s');
	});

	it('keeps a long session with recent activity in progress', () => {
		expect(
			sessionDurationLabel(
				{ startedAt: minutesAgo(40), duration: 35 * 60e9, hasRecording: true },
				now
			)
		).toBe('in progress');
	});

	it('treats a fresh session without segments as in progress', () => {
		expect(sessionDurationLabel({ startedAt: minutesAgo(5), duration: 0 }, now)).toBe(
			'in progress'
		);
	});

	it('labels an old session without segments or end as unrecorded', () => {
		expect(sessionDurationLabel({ startedAt: minutesAgo(40), duration: 0 }, now)).toBe(
			'No recording'
		);
	});
});
