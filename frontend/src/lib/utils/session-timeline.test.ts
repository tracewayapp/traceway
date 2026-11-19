import { describe, expect, it, vi } from 'vitest';

const get = vi.fn();
vi.mock('$lib/api', () => ({ api: { get: (...args: unknown[]) => get(...args) } }));

import {
	MAX_TIMELINE_WINDOW_MS,
	loadSessionTimeline,
	pickBreadcrumbs,
	timelineWindow
} from './session-timeline';

const log = {
	type: 'log' as const,
	timestamp: '2026-11-01T10:00:00Z',
	level: 'info' as const,
	message: 'm'
};

describe('session timeline', () => {
	it('clamps the window to what the backend accepts', () => {
		const from = Date.parse('2026-11-01T10:00:00Z');
		expect(timelineWindow(from, from + 10 * MAX_TIMELINE_WINDOW_MS).to).toBe(
			new Date(from + MAX_TIMELINE_WINDOW_MS).toISOString()
		);
		expect(timelineWindow(from, from - 1000).to).toBe(new Date(from).toISOString());
	});

	it('reads the timeline and treats a failed read as missing', async () => {
		get.mockResolvedValueOnce({ logs: [log], actions: [], truncated: true });
		const from = Date.parse('2026-11-01T10:00:00Z');
		expect(await loadSessionTimeline('s1', from, from + 60_000, 'p1')).toEqual({
			logs: [log],
			actions: [],
			truncated: true
		});
		expect(get).toHaveBeenCalledWith(
			'/sessions/s1/timeline?from=2026-11-01T10%3A00%3A00.000Z&to=2026-11-01T10%3A01%3A00.000Z',
			{ projectId: 'p1' }
		);

		get.mockRejectedValueOnce(new Error('boom'));
		expect(await loadSessionTimeline('s1', from, from + 60_000)).toBeNull();
	});

	it('anchors the read at the end of the window when asked', async () => {
		get.mockResolvedValueOnce({ logs: [log], actions: [], truncated: true });
		const at = Date.parse('2026-11-01T10:10:00Z');
		expect(await loadSessionTimeline('s1', at - 600_000, at, 'p1', 'end')).toEqual({
			logs: [log],
			actions: [],
			truncated: true
		});
		expect(get).toHaveBeenLastCalledWith(
			'/sessions/s1/timeline?from=2026-11-01T10%3A00%3A00.000Z&to=2026-11-01T10%3A10%3A00.000Z&anchor=end',
			{ projectId: 'p1' }
		);
	});

	it('clamps an end-anchored window from its start', () => {
		const to = Date.parse('2026-11-01T10:00:00Z');
		expect(timelineWindow(to - 10 * MAX_TIMELINE_WINDOW_MS, to, 'end')).toEqual({
			from: new Date(to - MAX_TIMELINE_WINDOW_MS).toISOString(),
			to: new Date(to).toISOString()
		});
	});

	it('retains legacy actions when only OTLP logs exist and sorts mixed logs', () => {
		const action = {
			type: 'custom' as const,
			timestamp: log.timestamp,
			category: 'ui',
			name: 'click'
		};
		const older = { ...log, timestamp: '2026-11-01T09:59:00Z' };
		expect(
			pickBreadcrumbs(
				{ logs: [older], actions: [], truncated: false },
				{ logs: [log], actions: [action] }
			)
		).toEqual({ logs: [older, log], actions: [action], truncated: false });
	});

	it('keeps legacy and OTLP breadcrumbs together', () => {
		const legacy = { logs: [{ ...log, message: 'legacy' }], actions: [] };
		expect(pickBreadcrumbs({ logs: [log], actions: [], truncated: false }, legacy).logs).toEqual([
			legacy.logs[0],
			log
		]);
		expect(pickBreadcrumbs({ logs: [], actions: [], truncated: false }, legacy).logs).toEqual(
			legacy.logs
		);
		expect(pickBreadcrumbs(null, legacy).logs).toEqual(legacy.logs);
		expect(pickBreadcrumbs({ logs: [log], actions: [], truncated: true }, { logs: [log] })).toEqual(
			{ logs: [log], actions: [], truncated: true }
		);
		expect(pickBreadcrumbs(null, null)).toEqual({ logs: [], actions: [], truncated: false });
	});
});
