import { filterOperator, operatorOption } from './attribute-filters';
import { parseLogAttributeFilter } from './log-filters';

export type SavedFilter = {
	id: number;
	projectId: string;
	page: string;
	name: string;
	query: string;
	createdBy: number | null;
	createdByName: string | null;
	createdAt: string;
	updatedAt: string;
};

export const TIME_PARAMS = ['preset', 'from', 'to'] as const;

// The URL params of /logs a saved filter keeps; the backend keeps the same
// list (savedFilterPageParams in saved_filter.controller.go).
export const LOGS_FILTER_PARAMS = [
	...TIME_PARAMS,
	'search',
	'searchType',
	'minSeverity',
	'service',
	'traceId',
	'spanId',
	'scopeName',
	'body',
	'attr',
	'chart',
	'chartView',
	'chartAgg'
] as const;

export function hasTimeRange(query: string): boolean {
	const params = new URLSearchParams(query);
	return TIME_PARAMS.some((p) => params.has(p));
}

// The page's own params in a fixed order (keys sorted, repeated values kept
// in order), so two queries compare equal whatever encoded them.
export function normalizeQuery(query: string | URLSearchParams, withTime = true): string {
	const params = typeof query === 'string' ? new URLSearchParams(query) : query;
	const out = new URLSearchParams();
	const keys = [...LOGS_FILTER_PARAMS]
		.filter((k) => withTime || !(TIME_PARAMS as readonly string[]).includes(k))
		.sort();
	for (const key of keys) {
		for (const value of params.getAll(key)) {
			if (value !== '') out.append(key, value);
		}
	}
	return out.toString();
}

// A saved filter without a time range matches whatever range is selected.
export function matchesSavedFilter(current: string, saved: SavedFilter): boolean {
	const withTime = hasTimeRange(saved.query);
	return normalizeQuery(current, withTime) === normalizeQuery(saved.query, withTime);
}

const severityLabels: Record<string, string> = {
	'1': 'TRACE+',
	'5': 'DEBUG+',
	'9': 'INFO+',
	'13': 'WARN+',
	'17': 'ERROR+',
	'21': 'FATAL'
};

export const presetLabels: Record<string, string> = {
	'5m': 'last 5m',
	'30m': 'last 30m',
	'60m': 'last 1h',
	'3h': 'last 3h',
	'6h': 'last 6h',
	'12h': 'last 12h',
	'24h': 'last 24h',
	'3d': 'last 3d',
	'7d': 'last 7d',
	'1M': 'last 30d',
	'3M': 'last 90d'
};

// Short parts for the saved filters menu: what the filter selects, in the
// order the logs page shows it.
export function describeSavedFilter(query: string): string[] {
	const params = new URLSearchParams(query);
	const parts: string[] = [];
	const search = params.get('search');
	if (search) parts.push(`"${search}"`);
	const severity = params.get('minSeverity');
	if (severity && severityLabels[severity]) parts.push(severityLabels[severity]);
	const service = params.get('service');
	if (service) parts.push(`service = ${service}`);
	for (const raw of params.getAll('attr')) {
		const f = parseLogAttributeFilter(raw);
		if (!f) continue;
		const op = operatorOption(filterOperator(f)).pill;
		parts.push(f.exists ? `${f.key} ${op}` : `${f.key} ${op} ${f.value}`);
	}
	if (params.get('traceId')) parts.push('one trace');
	if (params.get('body')) parts.push('exact message');
	const chart = params.get('chart');
	if (chart) {
		const key = chart.replace(/^(resource|scope|log)\./, '');
		parts.push(params.get('chartView') === 'distribution' ? `${key} distribution` : `${key} chart`);
	}
	const preset = params.get('preset');
	if (preset) parts.push(presetLabels[preset] ?? preset);
	else if (params.get('from') && params.get('to')) parts.push('fixed range');
	return parts;
}

// A param object as updateUrl takes it, as a query string.
export function toQueryString(
	params: Record<string, string | string[] | null | undefined>
): string {
	const query = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (Array.isArray(value)) value.forEach((v) => query.append(key, v));
		else if (value) query.set(key, value);
	}
	return query.toString();
}

// The params to apply a saved filter with: a filter saved without a time
// range keeps the range that is selected.
export function paramsToApply(
	filter: SavedFilter,
	selected: { preset: string | null; from: string; to: string }
): URLSearchParams {
	const params = new URLSearchParams(filter.query);
	if (!hasTimeRange(filter.query)) {
		if (selected.preset) {
			params.set('preset', selected.preset);
		} else {
			params.set('from', selected.from);
			params.set('to', selected.to);
		}
	}
	return params;
}
