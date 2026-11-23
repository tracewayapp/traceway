import { describe, expect, it } from 'vitest';
import {
	parseSpanAttributeFilter,
	parseSpanFilter,
	readSpanChart,
	readSpanSearchFilters,
	spanChartBody,
	spanChartUrlParams,
	spanExplorerHref,
	spanSearchBody,
	spanSearchUrlParams,
	spanTraceHref,
	summarizeSpanTrace
} from './span-explorer';
import type { Span } from '$lib/types/spans';

function urlOf(params: Record<string, string | string[] | undefined>): string {
	const search = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (Array.isArray(value)) value.forEach((item) => search.append(key, item));
		else if (value) search.set(key, value);
	}
	return `?${search}`;
}

describe('span search filters', () => {
	it('survive a trip through the URL', () => {
		const filters = {
			name: 'GET /checkout',
			service: 'cart',
			kind: '2',
			status: '2',
			minMs: '12.5',
			maxMs: '900',
			traceId: '0af7651916cd43dd8448eb211c80319c',
			attributes: [
				{
					key: 'http.request.method',
					value: 'GET',
					exclude: false,
					contains: false,
					exists: false
				},
				{ key: 'url.query', value: 'a=b&c=d', exclude: true, contains: false, exists: false },
				{ key: 'db.statement', value: 'orders', exclude: false, contains: true, exists: false },
				{ key: 'cache.hit', value: '', exclude: true, contains: false, exists: true },
				{ key: 'weird=key!', value: 'x', exclude: false, contains: false, exists: false }
			]
		};
		expect(readSpanSearchFilters(urlOf(spanSearchUrlParams(filters)))).toEqual(filters);
	});

	it('read links made before the operators', () => {
		expect(readSpanSearchFilters('?attr=url.path%3D%2Fcheckout').attributes).toEqual([
			{ key: 'url.path', value: '/checkout', exclude: false, contains: false, exists: false }
		]);
	});

	it('drop attribute entries that have no key or no value', () => {
		expect(readSpanSearchFilters('?attr=novalue&attr==x&attr=k=&attr=k!~=').attributes).toEqual([]);
		expect(readSpanSearchFilters('?attr=k?=').attributes).toHaveLength(1);
	});

	it('send the operators and no value for a key check', () => {
		const body = spanSearchBody(readSpanSearchFilters('?attr=cache.hit!%3F%3D&attr=db.rows~%3D5'));
		expect(body.attributeFilters).toEqual([
			{ key: 'cache.hit', value: '', exclude: true, contains: false, exists: true },
			{ key: 'db.rows', value: '5', exclude: false, contains: true, exists: false }
		]);
	});

	it('send only the filters that are set', () => {
		const body = spanSearchBody(readSpanSearchFilters('?status=0&maxMs=-3'));
		expect(body.status).toBe(0);
		expect(body.kind).toBeUndefined();
		expect(body.minDurationMs).toBeUndefined();
		expect(body.maxDurationMs).toBe(-3);
	});

	it('rejects malformed durations instead of silently broadening the search', () => {
		for (const value of ['abc', 'Infinity', '1e999']) {
			expect(() => spanSearchBody(readSpanSearchFilters(`?minMs=${value}`))).toThrow(
				/finite duration/
			);
		}
	});
});

describe('explorer filter dialog', () => {
	it('takes an operator and needs no value to check a key', () => {
		expect(parseSpanFilter(' cache.hit ', 'ignored', '!?=', [])).toEqual({
			key: 'cache.hit',
			value: '',
			exclude: true,
			contains: false,
			exists: true
		});
		expect(parseSpanFilter('db.rows', '', '~=', [])).toMatch(/value/);
	});

	it('lets an edited pill keep its place but not copy another', () => {
		const a = parseSpanFilter('a', '1', '=', []) as never;
		const b = parseSpanFilter('b', '2', '=', []) as never;
		expect(parseSpanFilter('a', '1', '=', [a, b], 0)).toEqual(a);
		expect(parseSpanFilter('b', '2', '=', [a, b], 0)).toMatch(/already/);
		const full = Array.from(
			{ length: 10 },
			(_, i) => parseSpanFilter(`k${i}`, 'v', '=', []) as never
		);
		expect(parseSpanFilter('k0', 'w', '=', full, 0)).not.toBeTypeOf('string');
		expect(parseSpanFilter('k', 'v', '=', full)).toMatch(/at most 10/);
	});
});

describe('explorer chart', () => {
	it('charts the duration unless the URL closes it', () => {
		expect(readSpanChart('')).toEqual({
			target: { kind: 'duration' },
			view: 'series',
			aggregations: ['p50', 'p95', 'p99']
		});
		expect(readSpanChart('?chart=none')).toBeNull();
		expect(spanChartUrlParams(null).chart).toBe('none');
		expect(spanChartUrlParams(readSpanChart(''))).toEqual({
			chart: undefined,
			chartView: undefined,
			chartAgg: undefined
		});
	});

	it('round trips an attribute chart', () => {
		const chart = readSpanChart('?chart=attr.db.rows&chartView=distribution&chartAgg=p95')!;
		expect(chart.target).toEqual({ kind: 'attribute', key: 'db.rows' });
		expect(spanChartUrlParams(chart)).toEqual({
			chart: 'attr.db.rows',
			chartView: 'distribution',
			chartAgg: 'p95'
		});
		expect(spanChartBody(chart.target)).toEqual({ attribute: 'db.rows' });
		expect(spanChartBody({ kind: 'duration' })).toEqual({});
	});
});

describe('links from a span to the explorer', () => {
	const span = {
		recordedAt: '2026-11-20T10:00:00Z',
		projectId: 'p2',
		name: 'SELECT orders',
		serviceName: 'checkout-api'
	};
	it('search the hour around the span in its own project', () => {
		const filter = {
			key: 'db.system',
			value: 'postgresql',
			exclude: false,
			contains: false,
			exists: false
		};
		const href = new URL(spanExplorerHref(span, { filter }, '?at=x'), 'http://x');
		expect(href.pathname).toBe('/spans');
		expect(href.searchParams.get('projectId')).toBe('p2');
		expect(href.searchParams.get('from')).toBe('2026-11-20T09:30:00.000Z');
		expect(href.searchParams.get('to')).toBe('2026-11-20T10:30:00.000Z');
		expect(href.searchParams.getAll('attr')).toEqual(['db.system=postgresql']);
	});
	it('keep the page time range and chart one operation', () => {
		const href = new URL(
			spanExplorerHref(span, { chartKey: 'db.rows' }, '?preset=24h'),
			'http://x'
		);
		expect(href.searchParams.get('preset')).toBe('24h');
		expect(href.searchParams.get('from')).toBeNull();
		expect(href.searchParams.get('chart')).toBe('attr.db.rows');
		expect(href.searchParams.get('name')).toBe('SELECT orders');
		expect(href.searchParams.get('service')).toBe('checkout-api');
	});
});

describe('SLO attribute filter', () => {
	it('trims and accepts a filter', () => {
		expect(parseSpanAttributeFilter(' url.path ', ' /checkout ', [])).toEqual({
			key: 'url.path',
			value: '/checkout'
		});
	});

	it('explains what is wrong', () => {
		const existing = [{ key: 'url.path', value: '/checkout' }];
		expect(parseSpanAttributeFilter('', 'x', [])).toMatch(/key/);
		expect(parseSpanAttributeFilter('a"b', 'x', [])).toMatch(/quotes/);
		expect(parseSpanAttributeFilter('url.path', '', [])).toMatch(/value/);
		expect(parseSpanAttributeFilter('url.path', '/checkout', existing)).toMatch(/already/);
		const full = Array.from({ length: 10 }, (_, i) => ({ key: `k${i}`, value: 'v' }));
		expect(parseSpanAttributeFilter('k', 'v', full)).toMatch(/at most 10/);
	});
});

describe('trace view', () => {
	const span = (overrides: Partial<Span>): Span => ({
		traceId: 'abc',
		spanId: '01',
		projectId: 'project',
		name: 'span',
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000_000,
		...overrides
	});

	it('links a span to its trace and keeps the time the read is bound to', () => {
		const href = spanTraceHref(span({ recordedAt: '2026-09-17T00:00:00.5Z' }), { focusSpan: true });
		expect(href).toBe('/spans/abc?at=2026-09-17T00%3A00%3A00.5Z&span=01');
		expect(spanTraceHref(span({ traceId: '' }))).toBeUndefined();
	});

	it.each(['1970-01-01T00:00:00Z', '2554-07-21T23:34:33.709551615Z'])(
		'opens spans with source time %s using their storage time',
		(startTime) => {
			const href = spanTraceHref(span({ startTime, recordedAt: '2026-09-20T12:34:56Z' }), {
				focusSpan: true
			});
			expect(href).toBe('/spans/abc?at=2026-09-20T12%3A34%3A56Z&span=01');
		}
	);

	it('measures the trace from its first start to its last end', () => {
		const summary = summarizeSpanTrace([
			span({ startTime: '2026-09-17T00:00:00.002Z', duration: 5_000_000, serviceName: 'api' }),
			span({ startTime: '2026-09-17T00:00:00.000Z', duration: 3_000_000, serviceName: 'web' }),
			span({ startTime: '2026-09-17T00:00:00.001Z', statusCode: 2, serviceName: 'api' })
		]);
		expect(summary).toEqual({
			startTime: '2026-09-17T00:00:00.000Z',
			duration: 7_000_000,
			services: 2,
			errors: 1
		});
		expect(summarizeSpanTrace([])).toEqual({ startTime: '', duration: 0, services: 0, errors: 0 });
	});
});
