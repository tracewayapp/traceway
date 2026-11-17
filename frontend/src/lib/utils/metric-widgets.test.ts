import { describe, expect, it } from 'vitest';
import {
	breakdownRows,
	isDistributionAggregation,
	needsWholeRange,
	isAdditiveAggregation,
	ratingShares
} from './metric-widgets';

const points = (...values: number[]) =>
	values.map((value, i) => ({ timestamp: `2026-11-15T10:0${i}:00Z`, value }));

describe('needsWholeRange', () => {
	it('asks for one value over the range where a time series cannot be reduced on the client', () => {
		expect(needsWholeRange('rating_bar', [])).toBe(true);
		expect(needsWholeRange('breakdown_table', undefined)).toBe(true);
		expect(
			needsWholeRange('single_value', [{ type: 'metric', name: 'lcp', aggregation: 'p75' }])
		).toBe(true);
		expect(
			needsWholeRange('gauge', [{ type: 'metric', name: 'lcp', aggregation: 'good_ratio' }])
		).toBe(true);
		expect(
			needsWholeRange('single_value', [{ type: 'metric', name: 'cpu', aggregation: 'avg' }])
		).toBe(false);
		expect(
			needsWholeRange('line_chart', [{ type: 'metric', name: 'lcp', aggregation: 'p75' }])
		).toBe(false);
	});

	it('knows which aggregations need a distribution', () => {
		expect(['p50', 'p99', 'good_ratio'].every(isDistributionAggregation)).toBe(true);
		expect(['avg', 'count', 'rate', '', undefined].some(isDistributionAggregation)).toBe(false);
	});
});

describe('ratingShares', () => {
	it('turns counts by rating into shares, in a fixed order', () => {
		const shares = ratingShares({
			poor: points(10, 15),
			good: points(50, 10),
			'needs-improvement': points(15),
			other: points(900)
		});
		expect(shares.map((s) => s.key)).toEqual(['good', 'needs-improvement', 'poor']);
		expect(shares.map((s) => s.count)).toEqual([60, 15, 25]);
		expect(shares.map((s) => s.share)).toEqual([0.6, 0.15, 0.25]);
	});

	it('has no shares without data', () => {
		expect(ratingShares({}).every((s) => s.share === 0 && s.count === 0)).toBe(true);
	});
});

describe('breakdownRows', () => {
	it('ranks groups by how many observations they have and keeps the top ones', () => {
		const rows = breakdownRows(
			{ '/a': points(1200), '/b': points(4000), '/c': points(900), '/d': points(100) },
			{ '/a': points(60), '/b': points(30), '/c': points(10) },
			3
		);
		expect(rows.map((r) => r.key)).toEqual(['/a', '/b', '/c']);
		expect(rows[1]).toEqual({ key: '/b', value: 4000, count: 30, share: 0.3 });
	});
});

describe('breakdownRows of a counter', () => {
	it('adds up the points of a group and ranks by the total', () => {
		const at = (value: number) => ({ timestamp: '2026-11-16T00:00:00Z', value });
		const rows = breakdownRows(
			{ 'img.hero': [at(3), at(4)], 'button#pay': [at(20)], footer: [at(1)] },
			{},
			2,
			true
		);
		expect(rows.map((row) => row.key)).toEqual(['button#pay', 'img.hero']);
		expect(rows[1]).toMatchObject({ value: 7, count: 7 });
		expect(rows[0].share).toBeCloseTo(20 / 28);
	});

	it('knows which aggregations add up', () => {
		expect(isAdditiveAggregation('sum')).toBe(true);
		expect(isAdditiveAggregation('count')).toBe(true);
		expect(isAdditiveAggregation('p75')).toBe(false);
		expect(isAdditiveAggregation(undefined)).toBe(false);
	});
});
