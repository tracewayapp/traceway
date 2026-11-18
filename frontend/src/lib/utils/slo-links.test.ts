import { describe, expect, it } from 'vitest';
import { badEventsHref } from './slo-links';
import { readSpanSearchFilters } from './span-explorer';

describe('badEventsHref', () => {
	it('opens the span explorer with the SLI filters and the error status', () => {
		const href = badEventsHref({
			sliKind: 'span',
			sliConfig: {
				serviceName: 'api',
				name: 'POST /checkout',
				kind: 2,
				attributes: [{ key: 'http.route', value: '/checkout' }],
				good: 'availability',
				latencyThresholdMs: 0
			}
		});
		expect(href.startsWith('/spans?')).toBe(true);
		const filters = readSpanSearchFilters(href.slice(href.indexOf('?')));
		expect(filters.service).toBe('api');
		expect(filters.name).toBe('POST /checkout');
		expect(filters.kind).toBe('2');
		expect(filters.status).toBe('2');
		expect(filters.attributes).toEqual([{ key: 'http.route', value: '/checkout' }]);
	});

	it('filters slow spans for a latency SLI', () => {
		const href = badEventsHref({
			sliKind: 'span',
			sliConfig: {
				serviceName: '',
				name: '',
				kind: null,
				attributes: [],
				good: 'latency',
				latencyThresholdMs: 800
			}
		});
		const filters = readSpanSearchFilters(href.slice(href.indexOf('?')));
		expect(filters.status).toBe('');
		expect(filters.minMs).toBe('800');
	});

	it('links the other kinds to their pages', () => {
		expect(
			badEventsHref({
				sliKind: 'endpoint',
				sliConfig: { endpoint: 'GET /users', good: 'availability', latencyThresholdMs: 0 }
			})
		).toBe('/endpoints/GET%20%2Fusers?preset=24h');
		expect(badEventsHref({ sliKind: 'task', sliConfig: { taskName: '' } })).toBe(
			'/tasks?preset=24h'
		);
		expect(badEventsHref({ sliKind: 'monitor', sliConfig: { checkIds: [7] } })).toBe('/monitors/7');
		expect(
			badEventsHref({ sliKind: 'session', sliConfig: { appVersion: '', includeMessages: false } })
		).toBe('/sessions?preset=24h');
		expect(
			badEventsHref({
				sliKind: 'metric',
				sliConfig: {
					metricName: 'x',
					tags: {},
					aggregation: 'avg',
					operator: 'lt',
					threshold: 1,
					missingIsBad: false
				}
			})
		).toBe('/dashboards');
	});
});
