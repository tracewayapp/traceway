import { presetGroups } from '$lib/utils/time-presets';
import type { FilterOption, SelectField, TextField } from './fields';

// Fields that several pages share, so the same filter reads and behaves the same everywhere.

// For "Applied to spans in the last 24 hours", in the picker's own words.
export function describeTimeRange(preset: string | null | undefined): string {
	if (!preset) return 'the custom range';
	const known = presetGroups.flat().find((p) => p.value === preset);
	return known ? `last ${known.label}` : preset;
}

const ROOT_OPTIONS: readonly FilterOption[] = [
	{ value: 'root', label: 'Root', pill: 'Root only' },
	{ value: 'non_root', label: 'Non-root', pill: 'Non-root only' }
];

// A root filter from a link or the bar, with anything unknown read as no filter.
export function rootFilterValue(value: string | null | undefined): string {
	return ROOT_OPTIONS.find((option) => option.value === value)?.value ?? 'all';
}

// noun: what a root or non-root row is on that page ("requests", "runs", "calls").
export function rootField(noun: string, key = 'rootFilter'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Root',
		anyLabel: 'Root and non-root',
		options: ROOT_OPTIONS,
		emptyValue: 'all',
		hint: `Non-root ${noun} were triggered by another trace.`
	};
}

// The values are what the endpoint search takes; `all` (no filter) is the field's empty value.
const METHOD_OPTIONS: readonly FilterOption[] = [
	{ value: 'get', label: 'GET', class: 'font-mono text-sky-700 dark:text-sky-400' },
	{ value: 'post', label: 'POST', class: 'font-mono text-emerald-700 dark:text-emerald-400' },
	{ value: 'put', label: 'PUT', class: 'font-mono text-amber-700 dark:text-amber-400' },
	{ value: 'patch', label: 'PATCH', class: 'font-mono text-orange-700 dark:text-orange-400' },
	{ value: 'delete', label: 'DELETE', class: 'font-mono text-rose-700 dark:text-rose-400' },
	{ value: 'options', label: 'OPTIONS', class: 'font-mono text-violet-700 dark:text-violet-400' },
	{ value: 'head', label: 'HEAD', class: 'font-mono text-violet-700 dark:text-violet-400' }
];

export function methodFilterValue(value: string | null | undefined): string {
	return METHOD_OPTIONS.find((option) => option.value === value)?.value ?? 'all';
}

export function methodField(key = 'methodFilter'): SelectField {
	return { kind: 'select', key, label: 'Method', options: METHOD_OPTIONS, emptyValue: 'all' };
}

export function issueTypeField(key = 'searchType'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Type',
		anyLabel: 'Issues and messages',
		emptyValue: 'all',
		inSearch: true,
		options: [
			{ value: 'issues', label: 'Issues', pill: 'Issues only' },
			{ value: 'messages', label: 'Messages', pill: 'Messages only' }
		]
	};
}

export function checkTypeField(key = 'type'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Type',
		options: [
			{ value: 'http', label: 'HTTP' },
			{ value: 'tcp', label: 'TCP' },
			{ value: 'browser', label: 'Browser' }
		]
	};
}

// What the backend accepts as a trace ID: 32 hex characters, dashes allowed, not all zeros.
export function traceIdError(value: string): string | null {
	const id = value.replaceAll('-', '').toLowerCase();
	return /^[0-9a-f]{32}$/.test(id) && /[^0]/.test(id)
		? null
		: 'A trace ID is 32 hexadecimal characters.';
}

export function traceIdField(key: string, hint: string): TextField {
	return {
		kind: 'text',
		key,
		label: 'Trace ID',
		placeholder: '32 hex characters',
		mono: true,
		hint,
		validate: traceIdError
	};
}
