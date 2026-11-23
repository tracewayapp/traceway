import { describe, expect, it } from 'vitest';
import {
	describeSavedFilter,
	hasTimeRange,
	matchesSavedFilter,
	normalizeQuery,
	type SavedFilter
} from './saved-filters';

const saved = (query: string): SavedFilter => ({
	id: 1,
	projectId: 'p',
	page: 'logs',
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
		expect(normalizeQuery(browser)).toBe(normalizeQuery(server));
		expect(normalizeQuery(browser)).not.toContain('projectId');
	});

	it('can leave the time range out', () => {
		expect(normalizeQuery('preset=24h&search=timeout', false)).toBe('search=timeout');
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
});

describe('describeSavedFilter', () => {
	it('lists what the filter selects', () => {
		expect(
			describeSavedFilter(
				'search=timeout&minSeverity=17&attr=log.NodeId%3D7&attr=log.Retry%3F%3D&chart=log.TimerSend&chartView=distribution&preset=24h'
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
});
