import { describe, expect, it } from 'vitest';
import {
	parseSpanAttributeFilter,
	readSpanSearchFilters,
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
				{ key: 'http.request.method', value: 'GET' },
				{ key: 'url.query', value: 'a=b&c=d' }
			]
		};
		expect(readSpanSearchFilters(urlOf(spanSearchUrlParams(filters)))).toEqual(filters);
	});

	it('drop attribute entries that have no key or no value', () => {
		expect(readSpanSearchFilters('?attr=novalue&attr==x&attr=k=').attributes).toEqual([]);
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

describe('attribute filter dialog', () => {
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
