import { describe, expect, it } from 'vitest';
import {
	budgetTone,
	burnTone,
	calculateSloInterval,
	clampRangeToWindow,
	defaultPresetForWindow,
	errorBudgetSeries,
	formatBudgetRemaining,
	formatSloPercent,
	sliSeries,
	sliSummary
} from './slo-math';

const buckets = [
	{ bucket: '2026-10-01T00:00:00Z', good: 990, total: 1000 },
	{ bucket: '2026-10-01T01:00:00Z', good: 1000, total: 1000 },
	{ bucket: '2026-10-01T02:00:00Z', good: 0, total: 0 },
	{ bucket: '2026-10-01T03:00:00Z', good: 980, total: 1000 }
];

describe('slo-math', () => {
	it('builds the SLI series from non-empty buckets only', () => {
		const points = sliSeries(buckets);
		expect(points).toHaveLength(3);
		expect(points[0].value).toBeCloseTo(99);
		expect(points[2].value).toBeCloseTo(98);
	});

	it('accumulates budget consumption against the range budget', () => {
		const points = errorBudgetSeries(buckets, 99);
		expect(points).toHaveLength(4);
		expect(points[0].value).toBeCloseTo((10 / 30) * 100);
		expect(points[3].value).toBeCloseTo(100);
	});

	it('has no budget series without traffic or without a budget', () => {
		expect(errorBudgetSeries([], 99)).toEqual([]);
		expect(errorBudgetSeries(buckets, 100)).toEqual([]);
	});

	it('tones the budget', () => {
		expect(budgetTone(null)).toBe('neutral');
		expect(budgetTone(-5)).toBe('danger');
		expect(budgetTone(10)).toBe('warning');
		expect(budgetTone(60)).toBe('success');
	});

	it('keeps at least the target precision', () => {
		expect(formatSloPercent(99.9512, 99.9)).toBe('99.95%');
		expect(formatSloPercent(99.9512, 99.999)).toBe('99.951%');
		expect(formatSloPercent(null, 99.9)).toBe('—');
		expect(formatSloPercent(99.8996, 99.9)).toBe('99.89%');
		expect(formatSloPercent(99.95, 99.9)).toBe('99.95%');
		expect(formatSloPercent(100, 99.9)).toBe('100.00%');
	});

	it('maps windows to presets and clamps ranges', () => {
		expect(defaultPresetForWindow(7)).toBe('7d');
		expect(defaultPresetForWindow(30)).toBe('1M');
		expect(defaultPresetForWindow(90)).toBe('3M');
		const now = new Date('2026-10-25T12:00:00Z');
		const clamped = clampRangeToWindow(new Date('2026-07-01T00:00:00Z'), now, 7, now);
		expect(clamped.clamped).toBe(true);
		expect(clamped.from.toISOString()).toBe('2026-10-18T12:00:00.000Z');
		const kept = clampRangeToWindow(new Date('2026-10-24T00:00:00Z'), now, 7, now);
		expect(kept.clamped).toBe(false);
		const endOfToday = new Date(now.getTime() + 11 * 60 * 60 * 1000);
		const trimmed = clampRangeToWindow(new Date('2026-10-24T00:00:00Z'), endOfToday, 7, now);
		expect(trimmed.clamped).toBe(false);
		expect(trimmed.to.toISOString()).toBe(now.toISOString());
	});

	it('never buckets finer than the SLO slices', () => {
		const from = new Date('2026-10-25T00:00:00Z');
		const to = new Date('2026-10-25T01:00:00Z');
		expect(calculateSloInterval(from, to, 1)).toBe(1);
		expect(calculateSloInterval(from, to, 15)).toBe(15);
		expect(calculateSloInterval(new Date('2026-07-25T00:00:00Z'), to, 1)).toBe(1440);
	});

	it('summarizes each kind', () => {
		expect(
			sliSummary({
				sliKind: 'endpoint',
				bucketMinutes: 1,
				sliConfig: { endpoint: '', good: 'latency', latencyThresholdMs: 500 }
			})
		).toBe('all endpoints · latency ≤ 500ms');
		expect(
			sliSummary({
				sliKind: 'span',
				bucketMinutes: 1,
				sliConfig: {
					serviceName: 'api',
					name: '',
					kind: 2,
					attributes: [{ key: 'http.route', value: '/x' }],
					good: 'availability',
					latencyThresholdMs: 0
				}
			})
		).toBe('api · Server · http.route=/x · availability');
		expect(
			sliSummary({
				sliKind: 'metric',
				bucketMinutes: 5,
				sliConfig: {
					metricName: 'cpu',
					tags: {},
					aggregation: 'avg',
					operator: 'lt',
					threshold: 90,
					missingIsBad: true
				}
			})
		).toBe('avg(cpu) < 90 · 5m slices');
		expect(
			sliSummary({ sliKind: 'monitor', bucketMinutes: 1, sliConfig: { checkIds: [1, 2] } })
		).toBe('2 monitors');
	});

	it('colours any burn that outpaces the window as a warning', () => {
		expect(burnTone(0.8)).toBe('success');
		expect(burnTone(1.3)).toBe('warning');
		expect(burnTone(14.4)).toBe('danger');
		expect(burnTone(null)).toBe('neutral');
	});

	it('shows an overspent budget at its real size', () => {
		expect(formatBudgetRemaining(33.94)).toBe('33.9%');
		expect(formatBudgetRemaining(-100)).toBe('-100%');
		expect(formatBudgetRemaining(-9900)).toBe('-9,900%');
		expect(formatBudgetRemaining(null)).toBe('—');
	});
});
