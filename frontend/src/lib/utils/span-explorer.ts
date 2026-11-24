import type { Span } from '$lib/types/spans';
import {
	formatAttributeFilter,
	keyOperatorOptions,
	parseAttributeFilter,
	type AttributeFilter
} from './attribute-filters';
import type { AttributeRules } from '$lib/components/traceway/filter-bar/attributes';
import { addStickyParamsToHref } from './navigation';
import { SPAN_STATUS_ERROR } from './span-fields';
import { timestampNanos } from './timestamp-nanos';
import {
	formatChartLines,
	parseChartLines,
	type ChartAggregation,
	type ChartLines
} from './value-chart';

// A search reads every span of the project in the range, so the explorer opens on a short one.
export const DEFAULT_SPAN_SEARCH_PRESET = '60m';
export const MAX_SPAN_ATTRIBUTE_FILTERS = 10;

// An exact attribute match, the only kind an SLO's span filters take.
export type SpanAttributeFilter = { key: string; value: string };

export type SpanSearchFilters = {
	name: string;
	service: string;
	kind: string;
	status: string;
	minMs: string;
	maxMs: string;
	traceId: string;
	attributes: AttributeFilter[];
};

// An existence filter matches on the key alone; every other operator needs a
// value, which the server insists on.
function usableFilter(filter: AttributeFilter | null): filter is AttributeFilter {
	return !!filter && (filter.exists || filter.value !== '');
}

export function spanTraceHref(
	span: Pick<Span, 'traceId' | 'spanId' | 'recordedAt'>,
	options: { focusSpan?: boolean } = {}
): string | undefined {
	if (!span.traceId) return undefined;
	const focus = options.focusSpan && span.spanId ? `&span=${span.spanId}` : '';
	return addStickyParamsToHref(
		`/spans/${span.traceId}?at=${encodeURIComponent(span.recordedAt)}${focus}`
	);
}

export function readSpanSearchFilters(search: string): SpanSearchFilters {
	const params = new URLSearchParams(search);
	const attributes = params
		.getAll('attr')
		.map(parseAttributeFilter)
		.filter(usableFilter)
		.slice(0, MAX_SPAN_ATTRIBUTE_FILTERS);
	return {
		name: params.get('name') ?? '',
		service: params.get('service') ?? '',
		kind: params.get('kind') ?? '',
		status: params.get('status') ?? '',
		minMs: params.get('minMs') ?? '',
		maxMs: params.get('maxMs') ?? '',
		traceId: params.get('trace') ?? '',
		attributes
	};
}

export function spanSearchUrlParams(
	filters: SpanSearchFilters
): Record<string, string | string[] | undefined> {
	return {
		name: filters.name || undefined,
		service: filters.service || undefined,
		kind: filters.kind || undefined,
		status: filters.status || undefined,
		minMs: filters.minMs.trim() || undefined,
		maxMs: filters.maxMs.trim() || undefined,
		trace: filters.traceId.trim() || undefined,
		attr: filters.attributes.map(formatAttributeFilter)
	};
}

function durationNumber(text: string): number | undefined {
	if (!text.trim()) return undefined;
	const value = Number(text.trim());
	if (!Number.isFinite(value)) {
		throw Object.assign(new Error('Enter a finite duration in milliseconds.'), { status: 422 });
	}
	return value;
}

export function spanSearchBody(filters: SpanSearchFilters) {
	return {
		name: filters.name.trim(),
		serviceName: filters.service,
		traceId: filters.traceId.trim(),
		kind: filters.kind === '' ? undefined : Number(filters.kind),
		status: filters.status === '' ? undefined : Number(filters.status),
		minDurationMs: durationNumber(filters.minMs),
		maxDurationMs: durationNumber(filters.maxMs),
		attributeFilters: filters.attributes.map(({ key, value, exclude, contains, exists }) => ({
			key,
			value: exists ? '' : value,
			exclude,
			contains,
			exists
		}))
	};
}

// What the span search accepts as an attribute filter.
export const SPAN_ATTRIBUTE_RULES: AttributeRules = {
	operators: keyOperatorOptions,
	hint: 'Keys are the OpenTelemetry names, such as http.request.method.',
	keyPlaceholder: 'http.request.method',
	valuePlaceholder: 'GET',
	value: 'trim',
	requireValue: true,
	max: MAX_SPAN_ATTRIBUTE_FILTERS,
	key: (key) =>
		/["\\]/.test(key) ? { error: 'Attribute keys cannot contain quotes or backslashes.' } : key
};

// What the chart above the explorer measures: the span duration, or one
// numeric attribute.
export type SpanChartTarget = { kind: 'duration' } | { kind: 'attribute'; key: string };

export type SpanChart = ChartLines & { target: SpanChartTarget };

export function spanChartDefaults(target: SpanChartTarget): readonly ChartAggregation[] {
	return target.kind === 'duration' ? ['p50', 'p95', 'p99'] : ['avg', 'max'];
}

export function spanChartLabel(target: SpanChartTarget): string {
	return target.kind === 'duration' ? 'Duration' : target.key;
}

// The duration chart is open unless the URL says `chart=none`; an attribute
// chart is `chart=attr.<key>`.
export function readSpanChart(search: string | URLSearchParams): SpanChart | null {
	const params = typeof search === 'string' ? new URLSearchParams(search) : search;
	const chart = params.get('chart');
	if (chart === 'none') return null;
	const target: SpanChartTarget =
		chart?.startsWith('attr.') && chart.length > 5
			? { kind: 'attribute', key: chart.slice(5) }
			: { kind: 'duration' };
	return { target, ...parseChartLines(params, spanChartDefaults(target)) };
}

export function spanChartUrlParams(chart: SpanChart | null): {
	chart: string | undefined;
	chartView: string | undefined;
	chartAgg: string | undefined;
} {
	if (!chart) return { chart: 'none', chartView: undefined, chartAgg: undefined };
	const lines = formatChartLines(chart, spanChartDefaults(chart.target));
	return {
		chart: chart.target.kind === 'attribute' ? `attr.${chart.target.key}` : undefined,
		chartView: lines.chartView ?? undefined,
		chartAgg: lines.chartAgg ?? undefined
	};
}

// The body of a chart request on top of the search's own filters.
export function spanChartBody(target: SpanChartTarget): { attribute?: string } {
	return target.kind === 'attribute' ? { attribute: target.key } : {};
}

// A link from a span (in a trace's waterfall) to the explorer: spans with the
// same attribute, or the chart of a numeric attribute across the spans of the
// same operation. The explorer keeps the page's time range when it has one, or
// else looks at the hour around the span. The span's own project is searched,
// since a whole trace can show spans of other projects.
export function spanExplorerHref(
	span: Pick<Span, 'recordedAt' | 'projectId' | 'name' | 'serviceName'>,
	target: { filter: AttributeFilter } | { chartKey: string },
	currentSearch: string
): string {
	const current = new URLSearchParams(currentSearch);
	const params = new URLSearchParams();
	if (span.projectId) params.set('projectId', span.projectId);
	if (current.get('preset')) {
		params.set('preset', current.get('preset')!);
	} else if (current.get('from') && current.get('to')) {
		params.set('from', current.get('from')!);
		params.set('to', current.get('to')!);
	} else {
		const at = new Date(span.recordedAt).getTime();
		params.set('from', new Date(at - 30 * 60_000).toISOString());
		params.set('to', new Date(at + 30 * 60_000).toISOString());
	}
	if ('filter' in target) {
		params.append('attr', formatAttributeFilter(target.filter));
	} else {
		if (span.serviceName) params.set('service', span.serviceName);
		if (span.name) params.set('name', span.name);
		params.set('chart', `attr.${target.chartKey}`);
	}
	return `/spans?${params}`;
}

export function parseSpanAttributeFilter(
	key: string,
	value: string,
	existing: SpanAttributeFilter[]
): SpanAttributeFilter | string {
	const filter = { key: key.trim(), value: value.trim() };
	if (!filter.key) return 'Enter an attribute key.';
	if (/["\\]/.test(filter.key)) return 'Attribute keys cannot contain quotes or backslashes.';
	if (!filter.value) return 'Enter the value to match.';
	if (existing.some((other) => other.key === filter.key && other.value === filter.value)) {
		return 'This filter is already applied.';
	}
	if (existing.length >= MAX_SPAN_ATTRIBUTE_FILTERS) {
		return `A search takes at most ${MAX_SPAN_ATTRIBUTE_FILTERS} attribute filters.`;
	}
	return filter;
}

export type SpanTraceSummary = {
	startTime: string;
	duration: number;
	services: number;
	errors: number;
};

export function summarizeSpanTrace(spans: Span[]): SpanTraceSummary {
	let first: Span | undefined;
	let start = 0n;
	let end = 0n;
	for (const span of spans) {
		const spanStart = timestampNanos(span.startTime);
		const spanEnd = spanStart + BigInt(Math.trunc(span.duration));
		if (!first || spanStart < start) {
			first = span;
			start = spanStart;
		}
		if (spanEnd > end) end = spanEnd;
	}
	return {
		startTime: first?.startTime ?? '',
		duration: first ? Number(end - start) : 0,
		services: new Set(spans.map((span) => span.serviceName).filter((name) => !!name)).size,
		errors: spans.filter((span) => span.statusCode === SPAN_STATUS_ERROR).length
	};
}
