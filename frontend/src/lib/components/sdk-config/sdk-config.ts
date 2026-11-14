export type Mode = 'off' | 'buffer' | 'stream';

export type ConditionOp =
	| 'eq'
	| 'neq'
	| 'in'
	| 'not_in'
	| 'contains'
	| 'icontains'
	| 'starts_with'
	| 'glob'
	| 'exists'
	| 'not_exists'
	| 'gt'
	| 'gte'
	| 'lt'
	| 'lte'
	| 'version';

export interface Condition {
	key: string;
	op: ConditionOp;
	value?: string | number | boolean;
	values?: Array<string | number | boolean>;
}

export interface Conditions {
	all?: Condition[];
	rollout?: { percent: number; by?: string };
}

export type SettingsDocument = Record<string, unknown> & {
	schemaVersion?: number;
	enabled?: boolean;
	signals?: Record<string, { mode?: Mode; sampleRate?: number; minSeverity?: string }>;
	sessions?: { enabled?: boolean; baselineSampleRate?: number };
	replay?: { mode?: Mode; sessionSampleRate?: number; maskAllInputs?: boolean };
	triggers?: unknown[];
};

export interface SdkConfigRule {
	id: number;
	name: string;
	position: number;
	enabled: boolean;
	conditions: Conditions;
	settings: SettingsDocument;
	updatedAt: string;
}

export interface SdkCapture {
	id: string;
	label: string;
	expiresAt: string;
	createdAt: string;
	active: boolean;
	status: string;
}

export const SIGNALS: Array<{ key: string; label: string; hint: string }> = [
	{ key: 'errors', label: 'Errors', hint: 'Uncaught errors and captureException' },
	{ key: 'messages', label: 'Messages', hint: 'captureMessage' },
	{ key: 'network', label: 'Network', hint: 'fetch and XHR spans' },
	{ key: 'console', label: 'Console', hint: 'console.* calls' },
	{ key: 'navigation', label: 'Navigation', hint: 'Route changes' },
	{ key: 'userActions', label: 'User actions', hint: 'Clicks' },
	{ key: 'webVitals', label: 'Web vitals', hint: 'LCP, INP, CLS, ...' },
	{ key: 'resourceTiming', label: 'Resource timing', hint: 'Page and asset load timings' },
	{ key: 'longTasks', label: 'Long tasks', hint: 'Main-thread blocks of 50 ms or more' },
	{ key: 'routeTiming', label: 'Route timing', hint: 'How long route changes take to render' },
	{ key: 'custom', label: 'Custom events', hint: 'track() and recordAction()' }
];

export const MODE_LABELS: Record<Mode, string> = {
	off: 'Off',
	buffer: 'Buffer',
	stream: 'Stream'
};

export const CONDITION_OPS: Array<{
	value: ConditionOp;
	label: string;
	needsValue: 'one' | 'many' | 'none';
}> = [
	{ value: 'eq', label: 'equals', needsValue: 'one' },
	{ value: 'neq', label: 'does not equal', needsValue: 'one' },
	{ value: 'in', label: 'is one of', needsValue: 'many' },
	{ value: 'not_in', label: 'is none of', needsValue: 'many' },
	{ value: 'contains', label: 'contains', needsValue: 'one' },
	{ value: 'icontains', label: 'contains (any case)', needsValue: 'one' },
	{ value: 'starts_with', label: 'starts with', needsValue: 'one' },
	{ value: 'glob', label: 'matches pattern (* ?)', needsValue: 'one' },
	{ value: 'version', label: 'version matches', needsValue: 'one' },
	{ value: 'gt', label: '>', needsValue: 'one' },
	{ value: 'gte', label: '≥', needsValue: 'one' },
	{ value: 'lt', label: '<', needsValue: 'one' },
	{ value: 'lte', label: '≤', needsValue: 'one' },
	{ value: 'exists', label: 'is set', needsValue: 'none' },
	{ value: 'not_exists', label: 'is not set', needsValue: 'none' }
];

export const SUGGESTED_KEYS = [
	'service.version',
	'deployment.environment.name',
	'browser.name',
	'browser.version',
	'browser.mobile',
	'os.name',
	'user.id',
	'session.id',
	'service.instance.id'
];

export const TRIGGER_TEMPLATES: Array<{ label: string; trigger: Record<string, unknown> }> = [
	{
		label: 'Error clip',
		trigger: {
			id: 'error-clip',
			name: 'Upload the last 30 seconds on an error',
			cooldownSeconds: 5,
			maxPerSession: 20,
			steps: [{ match: { key: 'event.name', op: 'eq', value: 'exception' } }],
			actions: [
				{ type: 'flush', lookbackSeconds: 600, signals: ['*'], replay: false },
				{ type: 'flush', lookbackSeconds: 30, signals: [], replay: true }
			]
		}
	},
	{
		label: 'Failed requests',
		trigger: {
			id: 'failed-requests',
			name: '3 failed requests within a minute',
			cooldownSeconds: 300,
			steps: [
				{
					match: { key: 'attributes.http.response.status_code', op: 'gte', value: 500 },
					count: 3,
					periodSeconds: 60
				}
			],
			actions: [
				{ type: 'flush', lookbackSeconds: 120, signals: ['*'], replay: true },
				{ type: 'stream', durationSeconds: 300, signals: ['*'], replay: true }
			]
		}
	},
	{
		label: 'Log line → capture',
		trigger: logLineTrigger({
			text: 'payment failed',
			op: 'icontains',
			minSeverity: 0,
			count: 1,
			periodSeconds: 60,
			durationMinutes: 30,
			lookbackSeconds: 120,
			replay: true
		})
	},
	{
		label: 'Error log burst → capture',
		trigger: {
			...logLineTrigger({
				text: '*',
				op: 'glob',
				minSeverity: 17,
				count: 5,
				periodSeconds: 60,
				durationMinutes: 15,
				lookbackSeconds: 300,
				replay: true
			}),
			id: 'error-log-burst',
			name: '5 error logs within a minute'
		}
	},
	{
		label: 'Frozen page → capture',
		trigger: performanceTrigger({
			id: 'frozen-page',
			name: 'The page froze for a second or more',
			signal: 'longTasks',
			attribute: 'browser.long_task.duration',
			thresholdMs: 1000
		})
	},
	{
		label: 'Slow route change → capture',
		trigger: performanceTrigger({
			id: 'slow-route-change',
			name: 'A route change took 3 seconds or more',
			signal: 'routeTiming',
			attribute: 'browser.route_timing.duration',
			thresholdMs: 3000
		})
	},
	{
		label: 'Custom event',
		trigger: {
			id: 'checkout-failed',
			name: 'Checkout failed',
			steps: [{ match: { key: 'event.name', op: 'eq', value: 'checkout_failed' } }],
			actions: [{ type: 'flush', lookbackSeconds: 300, signals: ['*'], replay: true }]
		}
	}
];

/**
 * "When this measurement reaches the threshold, capture this device for ten minutes." The signal is matched while
 * it is off, and it is off by default, so the capture names it next to `*` to have it sent.
 */
function performanceTrigger(o: {
	id: string;
	name: string;
	signal: 'longTasks' | 'routeTiming';
	attribute: string;
	thresholdMs: number;
}): Record<string, unknown> {
	return {
		id: o.id,
		name: o.name,
		cooldownSeconds: 300,
		steps: [
			{
				match: {
					all: [
						{ key: 'signal', op: 'eq', value: o.signal },
						{ key: `attributes.${o.attribute}`, op: 'gte', value: o.thresholdMs }
					]
				}
			}
		],
		actions: [
			{
				type: 'capture',
				durationSeconds: 600,
				lookbackSeconds: 120,
				signals: ['*', 'longTasks', 'routeTiming'],
				replay: true
			}
		]
	};
}

export type LogMatchOp = 'icontains' | 'glob' | 'starts_with';

export interface LogLineTriggerOptions {
	text: string;
	op: LogMatchOp;
	/** OpenTelemetry severity number the line must reach: 13 = warn, 17 = error; 0 = any. */
	minSeverity: number;
	count: number;
	periodSeconds: number;
	durationMinutes: number;
	lookbackSeconds: number;
	replay: boolean;
}

function slug(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 40);
}

/**
 * The trigger for "when this log line shows up (N times within M seconds), capture this device for Y minutes".
 * It matches every log record, console calls and the OpenTelemetry logs API alike, by the `console` signal.
 */
export function logLineTrigger(o: LogLineTriggerOptions): Record<string, unknown> {
	const conditions: Condition[] = [
		{ key: 'signal', op: 'eq', value: 'console' },
		{ key: 'body', op: o.op, value: o.text }
	];
	if (o.minSeverity > 0) conditions.push({ key: 'severity', op: 'gte', value: o.minSeverity });
	const step: Record<string, unknown> = { match: { all: conditions } };
	if (o.count > 1) {
		step.count = o.count;
		step.periodSeconds = o.periodSeconds;
	}
	const times = o.count > 1 ? ` ${o.count} times in ${o.periodSeconds} s` : '';
	return {
		id: `log-${slug(o.text) || 'line'}`,
		name: `Capture when "${o.text}" is logged${times}`,
		steps: [step],
		actions: [
			{
				type: 'capture',
				durationSeconds: o.durationMinutes * 60,
				lookbackSeconds: o.lookbackSeconds,
				signals: ['*'],
				replay: o.replay
			}
		]
	};
}

export function uniqueTriggerId(id: string, taken: Array<{ id?: unknown }>): string {
	const ids = new Set(taken.map((t) => t.id));
	if (!ids.has(id)) return id;
	for (let n = 2; ; n++) {
		const candidate = `${id.slice(0, 60)}-${n}`;
		if (!ids.has(candidate)) return candidate;
	}
}

export function summarizeConditions(conditions: Conditions): string {
	const parts = (conditions.all ?? []).map((c) => {
		const value = c.values ? c.values.join(', ') : c.value === undefined ? '' : String(c.value);
		const operator = CONDITION_OPS.find((operator) => operator.value === c.op)?.label ?? c.op;
		return `${c.key} ${operator}${value ? ` ${value}` : ''}`;
	});
	if (conditions.rollout) parts.push(`${conditions.rollout.percent}% of clients`);
	return parts.length > 0 ? parts.join(' and ') : 'All clients';
}

export function summarizeSettings(settings: SettingsDocument): string {
	const parts: string[] = [];
	if (settings.enabled === false) parts.push('collection off');
	for (const signal of SIGNALS) {
		const mode = settings.signals?.[signal.key]?.mode;
		if (mode) parts.push(`${signal.label.toLowerCase()}: ${MODE_LABELS[mode].toLowerCase()}`);
	}
	if (settings.replay?.mode)
		parts.push(`replay: ${MODE_LABELS[settings.replay.mode].toLowerCase()}`);
	if (settings.sessions?.baselineSampleRate !== undefined)
		parts.push(`${settings.sessions.baselineSampleRate * 100}% healthy session sample`);
	if (settings.sessions?.enabled === false) parts.push('sessions off');
	const triggers = settings.triggers?.length ?? 0;
	if (triggers > 0) parts.push(`${triggers} trigger${triggers === 1 ? '' : 's'}`);
	return parts.length > 0 ? parts.join(', ') : 'No changes';
}

/** Whether the form can show every field the document sets; anything more is edited as JSON. */
export function fitsForm(settings: SettingsDocument): boolean {
	if (!settings || typeof settings !== 'object' || Array.isArray(settings)) return false;
	const allowedTop = new Set(['schemaVersion', 'enabled', 'signals', 'sessions', 'replay']);
	if (Object.keys(settings).some((k) => !allowedTop.has(k))) return false;
	for (const value of Object.values(settings.signals ?? {})) {
		if (Object.keys(value ?? {}).some((k) => k !== 'mode')) return false;
	}
	return (
		Object.keys(settings.sessions ?? {}).every(
			(k) => k === 'enabled' || k === 'baselineSampleRate'
		) && Object.keys(settings.replay ?? {}).every((k) => k === 'mode')
	);
}

export type ConditionRow = { key: string; op: ConditionOp; value: string; original?: Condition };

export function conditionText(condition: Condition): string {
	return condition.values
		? condition.values.join(', ')
		: condition.value === undefined
			? ''
			: String(condition.value);
}

const NUMERIC_OPS: ReadonlySet<ConditionOp> = new Set(['gt', 'gte', 'lt', 'lte']);
const DECIMAL = /^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i;

export function isNumericOp(op: ConditionOp): boolean {
	return NUMERIC_OPS.has(op);
}

/** The server compares gt/gte/lt/lte against a JSON number, so a numeric text goes out as one. */
export function conditionValue(op: ConditionOp, text: string): string | number {
	const trimmed = text.trim();
	if (isNumericOp(op) && DECIMAL.test(trimmed)) {
		const number = Number(trimmed);
		if (Number.isFinite(number)) return number;
	}
	return trimmed;
}

/**
 * A cleared `<input type="number">` binds null (Svelte 5), and an unparsable one NaN; both mean "not set",
 * never 0.
 */
export function optionalNumber(value: number | string | null | undefined): number | undefined {
	if (value === null || value === undefined || (typeof value === 'string' && value.trim() === ''))
		return undefined;
	const number = Number(value);
	return Number.isFinite(number) ? number : undefined;
}

export function buildRuleConditions(
	rows: ConditionRow[],
	percent: number | string | null | undefined,
	by: string
): Conditions {
	const all = rows
		.filter((row) => row.key.trim() !== '')
		.map((row): Condition => {
			const original = row.original;
			const sameText = original !== undefined && row.value === conditionText(original);
			if (sameText && row.key === original.key && row.op === original.op)
				return {
					...original,
					...(original.values ? { values: [...original.values] } : {})
				};
			const condition: Condition = { key: row.key.trim(), op: row.op };
			const kind = CONDITION_OPS.find((op) => op.value === row.op)?.needsValue ?? 'one';
			if (kind === 'many')
				condition.values =
					sameText && original.values
						? [...original.values]
						: row.value
								.split(',')
								.map((value) => value.trim())
								.filter(Boolean);
			else if (kind === 'one')
				condition.value =
					sameText && keepsTypedValue(row.op, original.value)
						? original.value
						: conditionValue(row.op, row.value);
			return condition;
		});
	const conditions: Conditions = {};
	if (all.length) conditions.all = all;
	const rollout = optionalNumber(percent);
	if (rollout !== undefined)
		conditions.rollout = { percent: rollout, ...(by.trim() ? { by: by.trim() } : {}) };
	return conditions;
}

// A number or boolean the text still shows (one created through the CLI or the API) keeps its JSON type when
// the key or operator changes, since the server compares a number numerically and a string as text.
function keepsTypedValue(op: ConditionOp, value: Condition['value']): boolean {
	if (typeof value === 'number') return true;
	return typeof value === 'boolean' && !isNumericOp(op);
}
