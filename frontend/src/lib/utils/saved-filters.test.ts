import { describe, expect, it } from 'vitest';
import {
	describeSavedFilter,
	hasTimeRange,
	matchesSavedFilter,
	normalizeQuery,
	type SavedFilter
} from './saved-filters';

const saved = (query: string, page = 'logs'): SavedFilter => ({
	id: 1,
	projectId: 'p',
	page,
	name: 'n',
	query,
	createdBy: null,
	createdByName: null,
	createdAt: '',
	updatedAt: ''
});

describe('normalizeQuery', () => {
	it('keeps the page params in a fixed order, whatever encoded them', () => {
		const browser =
			'projectId=p1&preset=24h&attr=log.NodeId%3D7&attr=log.Label!%3Da&minSeverity=17';
		const server = 'attr=log.NodeId%3D7&attr=log.Label%21%3Da&minSeverity=17&preset=24h';
		expect(normalizeQuery(browser, 'logs')).toBe(normalizeQuery(server, 'logs'));
		expect(normalizeQuery(browser, 'logs')).not.toContain('projectId');
	});

	it('can leave the time range out', () => {
		expect(normalizeQuery('preset=24h&search=timeout', 'logs', false)).toBe('search=timeout');
	});

	it('keeps each page to its own params', () => {
		const query = 'preset=60m&name=checkout&search=timeout&minMs=40&kind=3';
		expect(normalizeQuery(query, 'spans')).toBe('kind=3&minMs=40&name=checkout&preset=60m');
		expect(normalizeQuery(query, 'logs')).toBe('preset=60m&search=timeout');
	});
});

describe('matchesSavedFilter', () => {
	it('matches a filter saved without a time range in any range', () => {
		const filter = saved('attr=log.NodeId%3D7');
		expect(matchesSavedFilter('preset=24h&attr=log.NodeId%3D7', filter)).toBe(true);
		expect(matchesSavedFilter('preset=7d&attr=log.NodeId%3D7', filter)).toBe(true);
		expect(matchesSavedFilter('preset=7d&attr=log.NodeId%3D8', filter)).toBe(false);
	});

	it('matches a filter saved with a range only in that range', () => {
		const filter = saved('preset=24h&attr=log.NodeId%3D7');
		expect(hasTimeRange(filter.query)).toBe(true);
		expect(matchesSavedFilter('preset=24h&attr=log.NodeId%3D7', filter)).toBe(true);
		expect(matchesSavedFilter('preset=7d&attr=log.NodeId%3D7', filter)).toBe(false);
	});

	it('tells filters with extra pills apart', () => {
		const filter = saved('attr=log.NodeId%3D7');
		expect(matchesSavedFilter('attr=log.NodeId%3D7&attr=log.Label%3Da', filter)).toBe(false);
	});

	it('compares a spans filter by the spans params', () => {
		const filter = saved('service=checkout-api&minMs=40', 'spans');
		expect(matchesSavedFilter('preset=60m&minMs=40&service=checkout-api', filter)).toBe(true);
		expect(matchesSavedFilter('preset=60m&minMs=40&service=inventory', filter)).toBe(false);
	});
});

describe('describeSavedFilter', () => {
	it('lists what the filter selects', () => {
		expect(
			describeSavedFilter(
				'search=timeout&minSeverity=17&attr=log.NodeId%3D7&attr=log.Retry%3F%3D&chart=log.TimerSend&chartView=distribution&preset=24h',
				'logs'
			)
		).toEqual([
			'"timeout"',
			'ERROR+',
			'NodeId = 7',
			'Retry exists',
			'TimerSend distribution',
			'last 24h'
		]);
	});

	it('lists what a spans filter selects', () => {
		expect(
			describeSavedFilter(
				'name=SELECT&service=checkout-api&kind=3&status=2&minMs=40&attr=db.system%3Dpostgresql&attr=cache.hit!%3F%3D&chart=attr.db.rows&chartView=distribution&preset=60m',
				'spans'
			)
		).toEqual([
			'"SELECT"',
			'service = checkout-api',
			'Client',
			'status Error',
			'≥ 40 ms',
			'db.system = postgresql',
			'cache.hit not exists',
			'db.rows distribution',
			'last 1h'
		]);
		expect(describeSavedFilter('chartView=distribution', 'spans')).toEqual([
			'duration distribution'
		]);
	});
});
