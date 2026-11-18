import { describe, expect, it } from 'vitest';
import { formatSessionDuration, sessionDurationLabel } from './session-duration';

const now = Date.parse('2026-09-26T17:00:00Z');
const minutesAgo = (m: number) => new Date(now - m * 60_000).toISOString();

describe('sessionDurationLabel', () => {
	it('shows the duration of an ended session', () => {
		expect(
			sessionDurationLabel(
				{ startedAt: minutesAgo(40), endedAt: minutesAgo(38), duration: 120e9 },
				now
			)
		).toBe('2m 0s');
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

describe('formatSessionDuration', () => {
	it.each([
		[11.9e9, '11.9s'],
		[59.94e9, '59.9s'],
		[60e9, '1m 0s'],
		[1267e9, '21m 7s'],
		[3600e9, '1h 0m'],
		[7514e9, '2h 5m'],
		[-5e9, '0.0s']
	])('formats %d ns as %s', (ns, label) => {
		expect(formatSessionDuration(ns)).toBe(label);
	});
});
