import type { Span } from '$lib/types/spans';
import { addStickyParamsToHref } from './navigation';
import { SPAN_STATUS_ERROR } from './span-fields';
import { timestampNanos } from './timestamp-nanos';

// A search reads every span of the project in the range, so the explorer opens on a short one.
export const DEFAULT_SPAN_SEARCH_PRESET = '60m';
export const MAX_SPAN_ATTRIBUTE_FILTERS = 10;

export type SpanAttributeFilter = { key: string; value: string };

export type SpanSearchFilters = {
	name: string;
	service: string;
	kind: string;
	status: string;
	minMs: string;
	maxMs: string;
	traceId: string;
	attributes: SpanAttributeFilter[];
};

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
		.map((entry) => {
			const split = entry.indexOf('=');
			return split > 0 ? { key: entry.slice(0, split), value: entry.slice(split + 1) } : null;
		})
		.filter((filter): filter is SpanAttributeFilter => !!filter && !!filter.value)
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
		attr: filters.attributes.map((filter) => `${filter.key}=${filter.value}`)
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
		attributeFilters: filters.attributes
	};
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
