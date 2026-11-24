import { METHOD_OPTIONS } from '$lib/components/ui/endpoint-filter';
import { presetLabels } from '$lib/utils/saved-filters';
import type { SelectField, TextField } from './fields';

// Fields that several pages share, so the same filter reads and behaves the same everywhere.

// Worded like the time range picker ("Last 24 hours"), for "Applied to spans in the last 24 hours".
const RANGE_WORDS: Record<string, string> = {
	'5m': '5 minutes',
	'30m': '30 minutes',
	'60m': '60 minutes',
	'3h': '3 hours',
	'6h': '6 hours',
	'12h': '12 hours',
	'24h': '24 hours',
	'3d': '3 days',
	'7d': '7 days',
	'1M': 'month',
	'3M': '3 months'
};

export function describeTimeRange(preset: string | null | undefined): string {
	if (!preset) return 'the custom range';
	return RANGE_WORDS[preset] ? `last ${RANGE_WORDS[preset]}` : (presetLabels[preset] ?? preset);
}

// noun: what a root or non-root row is on that page ("requests", "runs", "calls").
export function rootField(noun: string, key = 'rootFilter'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Root',
		anyLabel: 'Root and non-root',
		options: [
			{ value: 'root', label: 'Root', pill: 'Root only' },
			{ value: 'non_root', label: 'Non-root', pill: 'Non-root only' }
		],
		emptyValue: 'all',
		hint: `Non-root ${noun} were triggered by another trace.`
	};
}

const METHOD_COLORS: Record<string, string> = {
	GET: 'font-mono text-sky-700 dark:text-sky-400',
	POST: 'font-mono text-emerald-700 dark:text-emerald-400',
	PUT: 'font-mono text-amber-700 dark:text-amber-400',
	PATCH: 'font-mono text-orange-700 dark:text-orange-400',
	DELETE: 'font-mono text-rose-700 dark:text-rose-400',
	HEAD: 'font-mono text-violet-700 dark:text-violet-400',
	OPTIONS: 'font-mono text-violet-700 dark:text-violet-400'
};

export function methodField(key = 'methodFilter'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Method',
		options: METHOD_OPTIONS.map((o) => ({ ...o, class: METHOD_COLORS[o.label] })),
		emptyValue: 'all'
	};
}

export function issueTypeField(key = 'searchType'): SelectField {
	return {
		kind: 'select',
		key,
		label: 'Type',
		anyLabel: 'Issues and messages',
		emptyValue: 'all',
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
