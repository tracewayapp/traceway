import type { Span } from '$lib/types/spans';

export const SPAN_KIND_OPTIONS = [
	{ value: '1', label: 'Internal' },
	{ value: '2', label: 'Server' },
	{ value: '3', label: 'Client' },
	{ value: '4', label: 'Producer' },
	{ value: '5', label: 'Consumer' }
];

export const SPAN_STATUS_OPTIONS = [
	{ value: '2', label: 'Error' },
	{ value: '1', label: 'Ok' },
	{ value: '0', label: 'Unset' }
];

export const SPAN_STATUS_ERROR = 2;

export function spanKindLabel(kind: number | undefined): string {
	return SPAN_KIND_OPTIONS.find((option) => option.value === String(kind))?.label ?? '';
}

export function spanStatusLabel(status: number | undefined): string {
	return SPAN_STATUS_OPTIONS.find((option) => option.value === String(status ?? 0))?.label ?? '';
}

export function isErrorSpan(span: Pick<Span, 'statusCode'>): boolean {
	return span.statusCode === SPAN_STATUS_ERROR;
}

export function spanServices(spans: Span[]): string[] {
	return [...new Set(spans.map((span) => span.serviceName).filter((name) => !!name))].sort((a, b) =>
		a!.localeCompare(b!)
	) as string[];
}
