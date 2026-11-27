import { describe, expect, it } from 'vitest';
import {
	buildRuleConditions,
	conditionText,
	conditionValue,
	fitsForm,
	jsonOnlyFields,
	logLineTrigger,
	mergeFormSettings,
	newTriggerId,
	triggerIdsOfRules,
	withTrigger,
	BUILT_IN_ERROR_TRIGGER,
	type SdkConfigRule,
	optionalNumber,
	REPLAY_IMAGES_HINTS,
	REPLAY_IMAGES_LABELS,
	sdkPlatform,
	SDK_PLATFORM_NAMES,
	signalsFor,
	suggestedKeysFor,
	SIGNALS,
	summarizeConditions,
	summarizeSettings,
	TRIGGER_TEMPLATES,
	uniqueTriggerId,
	type Condition,
	type ConditionRow
} from './sdk-config';

describe('sdk-config helpers', () => {
	it('summarizes conditions and rollout', () => {
		expect(summarizeConditions({})).toBe('All clients');
		expect(
			summarizeConditions({
				all: [
					{ key: 'service.version', op: 'version', value: '>=2.4' },
					{ key: 'browser.name', op: 'in', values: ['Chrome', 'Edge'] }
				],
				rollout: { percent: 10 }
			})
		).toBe(
			'service.version version matches >=2.4 and browser.name is one of Chrome, Edge and 10% of clients'
		);
	});

	it('summarizes settings', () => {
		expect(summarizeSettings({ schemaVersion: 1 })).toBe('No changes');
		expect(
			summarizeSettings({
				signals: { network: { mode: 'off' } },
				replay: { mode: 'stream' },
				triggers: [{}, {}]
			})
		).toBe('network: off, replay: stream, 2 triggers');
	});

	it('names a minimum severity in the summary', () => {
		expect(
			summarizeSettings({ signals: { console: { mode: 'buffer', minSeverity: 'warn' } } })
		).toBe('console: buffer (warn or worse)');
		expect(summarizeSettings({ signals: { console: { minSeverity: 'error' } } })).toBe(
			'console: error or worse'
		);
		expect(
			summarizeSettings({
				signals: { console: { mode: 'stream' }, messages: { mode: 'off', minSeverity: 'info' } }
			})
		).toBe('messages: off (info or worse), console: stream');
	});

	it('names the replay type in the summary', () => {
		expect(summarizeSettings({ replay: { captureMode: 'wireframe' } })).toBe('replay as wireframe');
		expect(summarizeSettings({ replay: { mode: 'stream', captureMode: 'video' } })).toBe(
			'replay: stream, replay as video'
		);
	});

	it('opens every settings object in the form, whatever fields only the JSON tab shows', () => {
		expect(
			fitsForm({ schemaVersion: 1, signals: { console: { mode: 'stream' } }, enabled: false })
		).toBe(true);
		expect(fitsForm({ signals: { console: { mode: 'stream', minSeverity: 'warn' } } })).toBe(true);
		expect(fitsForm({ signals: { custom: { sampleRate: 1 } } })).toBe(true);
		expect(fitsForm({ triggers: [] })).toBe(true);
		expect(fitsForm({ sessions: { enabled: true, baselineSampleRate: 0.01 } })).toBe(true);
		expect(fitsForm({ replay: { mode: 'buffer', sessionSampleRate: 0.5 } })).toBe(true);
		expect(fitsForm({ buffer: { maxBytes: 1024 } })).toBe(true);
	});

	it('keeps documents that are not a settings object in the JSON tab', () => {
		expect(fitsForm([] as never)).toBe(false);
		expect(fitsForm({ triggers: {} } as never)).toBe(false);
		expect(fitsForm({ replay: 'stream' } as never)).toBe(false);
		expect(fitsForm({ signals: { custom: 'stream' } } as never)).toBe(false);
	});

	it('names the fields only the JSON tab shows', () => {
		expect(
			jsonOnlyFields({
				schemaVersion: 1,
				signals: { custom: { mode: 'stream', sampleRate: 1 }, console: { mode: 'buffer' } },
				replay: { images: 'wifi', maskAllInputs: true },
				buffer: { maxBytes: 1024 },
				triggers: [{ id: 'x' }]
			})
		).toEqual(['signals.custom.sampleRate', 'replay.maskAllInputs', 'buffer.maxBytes']);
		expect(
			jsonOnlyFields({ schemaVersion: 1, enabled: false, sessions: { enabled: true } })
		).toEqual([]);
	});

	it('saves the form over the loaded document, keeping the JSON-only fields untouched', () => {
		const loaded = {
			schemaVersion: 1,
			signals: { custom: { sampleRate: 1 }, console: { mode: 'stream', minSeverity: 'warn' } },
			sessions: { enabled: true },
			replay: { mode: 'buffer', sessionSampleRate: 0.5 },
			metrics: { sampleRate: 0.5, flushSeconds: 60 },
			buffer: { maxBytes: 1024 },
			triggers: [{ id: 'old' }]
		} as const;
		const form = {
			schemaVersion: 1,
			signals: { custom: { mode: 'off' as const } },
			replay: { images: 'off' as const },
			triggers: [{ id: 'new' }]
		};
		expect(mergeFormSettings(structuredClone(loaded) as never, form)).toEqual({
			schemaVersion: 1,
			signals: { custom: { sampleRate: 1, mode: 'off' }, console: { minSeverity: 'warn' } },
			replay: { sessionSampleRate: 0.5, images: 'off' },
			metrics: { flushSeconds: 60 },
			buffer: { maxBytes: 1024 },
			triggers: [{ id: 'new' }]
		});
	});

	it('round-trips a rule saved from the form without changes', () => {
		const loaded = {
			schemaVersion: 1,
			signals: { custom: { mode: 'stream' as const, sampleRate: 1 } },
			triggers: [{ id: 'e2e' }]
		};
		const unchanged = {
			schemaVersion: 1,
			signals: { custom: { mode: 'stream' as const } },
			triggers: [{ id: 'e2e' }]
		};
		expect(mergeFormSettings(loaded, unchanged)).toEqual(loaded);
		expect(loaded.signals.custom).toEqual({ mode: 'stream', sampleRate: 1 });
	});

	it('names the replay image policy in the summary', () => {
		expect(summarizeSettings({ replay: { images: 'off' } })).toBe('replay images off');
		expect(summarizeSettings({ replay: { mode: 'buffer', images: 'wifi' } })).toBe(
			'replay: buffer, replay images on Wi‑Fi only'
		);
		expect(summarizeSettings({ replay: { captureMode: 'wireframe', images: 'always' } })).toBe(
			'replay as wireframe, replay images always'
		);
	});

	it('edits the replay image policy in the form', () => {
		expect(fitsForm({ replay: { images: 'wifi' } })).toBe(true);
		expect(fitsForm({ replay: { mode: 'stream', captureMode: 'wireframe', images: 'off' } })).toBe(
			true
		);
		expect(Object.keys(REPLAY_IMAGES_LABELS)).toEqual(['off', 'wifi', 'always']);
		expect(Object.values(REPLAY_IMAGES_LABELS)).toEqual(['Off', 'Wi‑Fi only', 'Always']);
	});

	it('describes replay images in the terms of each SDK', () => {
		expect(REPLAY_IMAGES_HINTS.browser).toBe('Off hides pictures; Always adds small copies');
		expect(REPLAY_IMAGES_HINTS.flutter).toBe('Small pictures of images, never a screenshot');
		expect(REPLAY_IMAGES_HINTS.ios).toBe('Pictures of image views, never a screenshot');
		expect(REPLAY_IMAGES_HINTS.android).toBe(REPLAY_IMAGES_HINTS.ios);
	});

	it('shows the replay type in the form, next to the replay mode', () => {
		expect(fitsForm({ replay: { captureMode: 'wireframe' } })).toBe(true);
		expect(fitsForm({ replay: { mode: 'stream', captureMode: 'video' } })).toBe(true);
	});
});

describe('log-line triggers', () => {
	it('builds a capture trigger for a log line', () => {
		const trigger = logLineTrigger({
			text: 'Payment failed!',
			op: 'icontains',
			minSeverity: 17,
			count: 3,
			periodSeconds: 60,
			durationMinutes: 30,
			lookbackSeconds: 300,
			replay: true
		});
		expect(trigger).toEqual({
			id: 'log-payment-failed',
			name: 'Capture when "Payment failed!" is logged 3 times in 60 s',
			steps: [
				{
					match: {
						all: [
							{ key: 'signal', op: 'in', values: ['console', 'messages'] },
							{ key: 'body', op: 'icontains', value: 'Payment failed!' },
							{ key: 'severity', op: 'gte', value: 17 }
						]
					},
					count: 3,
					periodSeconds: 60
				}
			],
			actions: [
				{
					type: 'capture',
					durationSeconds: 1800,
					lookbackSeconds: 300,
					signals: ['*'],
					replay: true
				}
			]
		});
	});

	it('names the performance signals in the captures they start, since `*` leaves out what is off by default', () => {
		const frozen = TRIGGER_TEMPLATES.find((t) => t.trigger.id === 'frozen-page')!.trigger;
		expect(frozen.steps).toEqual([
			{
				match: {
					all: [
						{ key: 'signal', op: 'eq', value: 'longTasks' },
						{ key: 'attributes.browser.long_task.duration', op: 'gte', value: 1000 }
					]
				}
			}
		]);
		for (const id of ['frozen-page', 'slow-route-change']) {
			const actions = TRIGGER_TEMPLATES.find((t) => t.trigger.id === id)!.trigger.actions as Array<{
				signals: string[];
			}>;
			expect(actions[0].signals).toEqual(['*', 'longTasks', 'routeTiming']);
		}
	});

	it('suggests the attributes the project’s SDK reports', () => {
		expect(suggestedKeysFor('browser')).toContain('browser.name');
		for (const platform of ['flutter', 'ios', 'android'] as const) {
			const keys = suggestedKeysFor(platform);
			expect(keys.some((key) => key.startsWith('browser.'))).toBe(false);
			expect(keys).toEqual(expect.arrayContaining(['os.name', 'device.form_factor', 'user.id']));
		}
	});
	it('offers a Flutter project only the signals its SDK reports, in its terms', () => {
		expect(signalsFor('browser')).toBe(SIGNALS);
		const flutter = signalsFor('flutter');
		expect(flutter.map((signal) => signal.key)).toEqual([
			'errors',
			'messages',
			'network',
			'console',
			'navigation',
			'userActions',
			'custom'
		]);
		expect(flutter.find((signal) => signal.key === 'userActions')?.hint).toBe('Taps');
		expect(SIGNALS.find((signal) => signal.key === 'userActions')?.hint).toBe('Clicks');
		const browserOnly = TRIGGER_TEMPLATES.filter((t) => t.browserOnly).map((t) => t.trigger.id);
		expect(browserOnly).toEqual(['frozen-page', 'slow-route-change']);
	});

	it('offers an iOS project the app signals, in its terms', () => {
		const ios = signalsFor('ios');
		expect(ios.map((signal) => signal.key)).toEqual(
			signalsFor('flutter').map((signal) => signal.key)
		);
		expect(ios.find((signal) => signal.key === 'console')?.hint).toBe(
			'os_log, Logger and NSLog lines, Traceway.log, and print with captureStdout'
		);
		expect(ios.find((signal) => signal.key === 'network')?.hint).toBe('URLSession requests');
		expect(sdkPlatform('ios')).toBe('ios');
		expect(sdkPlatform('flutter')).toBe('flutter');
		expect(sdkPlatform('react')).toBe('browser');
		expect(SDK_PLATFORM_NAMES[sdkPlatform('ios')]).toBe('iOS');
	});

	it('offers an Android project the app signals, in its terms', () => {
		const android = signalsFor('android');
		expect(android.map((signal) => signal.key)).toEqual(
			signalsFor('flutter').map((signal) => signal.key)
		);
		expect(android.find((signal) => signal.key === 'console')?.hint).toBe(
			'Logcat lines with the Gradle plugin, System.out/err and Traceway.log'
		);
		expect(android.find((signal) => signal.key === 'network')?.hint).toBe(
			'OkHttp calls and startRequest/endRequest'
		);
		expect(sdkPlatform('android')).toBe('android');
		expect(SDK_PLATFORM_NAMES[sdkPlatform('android')]).toBe('Android');
	});

	it('gives a template an id no other rule of the project uses', () => {
		const rules = [
			{ id: 1, settings: { triggers: [{ id: 'frozen-page' }, { id: BUILT_IN_ERROR_TRIGGER }] } },
			{ id: 2, settings: { triggers: [{ id: 'frozen-page-2' }] } },
			{ id: 3, settings: {} }
		] as unknown as SdkConfigRule[];
		expect(triggerIdsOfRules(rules, 2)).toEqual(['frozen-page']);
		const others = triggerIdsOfRules(rules, 3);
		expect(others.sort()).toEqual(['frozen-page', 'frozen-page-2']);
		expect(withTrigger([], { id: 'frozen-page', steps: [] }, others).map((t) => t.id)).toEqual([
			'frozen-page-3'
		]);
	});

	it('replaces the built-in error trigger instead of adding a second one', () => {
		const template = TRIGGER_TEMPLATES.find((t) => t.trigger.id === BUILT_IN_ERROR_TRIGGER)!;
		const once = withTrigger([{ id: 'other' }], template.trigger, [BUILT_IN_ERROR_TRIGGER]);
		expect(once.map((t) => t.id)).toEqual(['other', BUILT_IN_ERROR_TRIGGER]);
		const twice = withTrigger(once, { ...template.trigger, cooldownSeconds: 60 });
		expect(twice.map((t) => t.id)).toEqual(['other', BUILT_IN_ERROR_TRIGGER]);
		expect((twice[1] as { cooldownSeconds: number }).cooldownSeconds).toBe(60);
		expect(TRIGGER_TEMPLATES.some((t) => t.trigger.id === 'error-clip')).toBe(false);
	});

	it('makes trigger ids the server accepts, without needing a secure context', () => {
		const id = newTriggerId();
		expect(id).toMatch(/^[A-Za-z0-9_.-]{1,64}$/);
		expect(newTriggerId()).not.toBe(id);
	});

	it('keeps trigger ids unique', () => {
		expect(uniqueTriggerId('log-x', [])).toBe('log-x');
		expect(uniqueTriggerId('log-x', [{ id: 'log-x' }, { id: 'log-x-2' }])).toBe('log-x-3');
	});
});

describe('rule conditions', () => {
	it('preserves identifiers and typed values when only the rule name changes', () => {
		const conditions = [
			{ key: 'user.id', op: 'eq' as const, value: '00123' },
			{ key: 'tenant', op: 'in' as const, values: ['a,b', 12, true] }
		];
		const rows = conditions.map((original) => ({
			key: original.key,
			op: original.op,
			value: conditionText(original),
			original
		}));
		expect(buildRuleConditions(rows, 25, 'user.id')).toEqual({
			all: conditions,
			rollout: { percent: 25, by: 'user.id' }
		});
	});
	it('accepts an empty or zero numeric rollout input', () => {
		expect(buildRuleConditions([], undefined, '')).toEqual({});
		expect(buildRuleConditions([], 0, '')).toEqual({ rollout: { percent: 0 } });
	});

	it('treats a cleared rollout input as no rollout, not 0%', () => {
		expect(buildRuleConditions([], null, 'user.id')).toEqual({});
		expect(buildRuleConditions([], Number.NaN, '')).toEqual({});
		expect(buildRuleConditions([], '', '')).toEqual({});
		expect(buildRuleConditions([], 12.5, '')).toEqual({ rollout: { percent: 12.5 } });
	});

	it('sends numeric comparisons as numbers', () => {
		const rows: ConditionRow[] = [
			{ key: 'browser.version', op: 'gte', value: ' 120 ' },
			{ key: 'screen.width', op: 'lt', value: '1.5e3' },
			{ key: 'cpu.cores', op: 'gt', value: '-2' },
			{ key: 'memory', op: 'lte', value: '.5' },
			{ key: 'user.id', op: 'eq', value: '00123' }
		];
		expect(buildRuleConditions(rows, undefined, '').all).toEqual([
			{ key: 'browser.version', op: 'gte', value: 120 },
			{ key: 'screen.width', op: 'lt', value: 1500 },
			{ key: 'cpu.cores', op: 'gt', value: -2 },
			{ key: 'memory', op: 'lte', value: 0.5 },
			{ key: 'user.id', op: 'eq', value: '00123' }
		]);
	});

	it('leaves a non-numeric comparison value as text for the server to reject', () => {
		expect(conditionValue('gte', 'abc')).toBe('abc');
		expect(conditionValue('gte', '0x10')).toBe('0x10');
		expect(conditionValue('gte', 'Infinity')).toBe('Infinity');
		expect(conditionValue('gte', '')).toBe('');
		expect(conditionValue('contains', '120')).toBe('120');
	});

	it('keeps the JSON type of values created through the CLI when other fields change', () => {
		const row = (original: Condition, change: Partial<ConditionRow>): ConditionRow => ({
			key: original.key,
			op: original.op,
			value: conditionText(original),
			original,
			...change
		});
		const width: Condition = { key: 'screen.width', op: 'eq', value: 1280 };
		const mobile: Condition = { key: 'browser.mobile', op: 'eq', value: true };
		const tenants: Condition = { key: 'tenant', op: 'in', values: [12, true, 'a'] };
		expect(
			buildRuleConditions(
				[
					row(width, { key: 'viewport.width' }),
					row(width, { op: 'gte' }),
					row(width, { value: '1440' }),
					row(mobile, { key: 'device.mobile' }),
					row(mobile, { op: 'gte' }),
					row(tenants, { key: 'org' })
				],
				undefined,
				''
			).all
		).toEqual([
			{ key: 'viewport.width', op: 'eq', value: 1280 },
			{ key: 'screen.width', op: 'gte', value: 1280 },
			{ key: 'screen.width', op: 'eq', value: '1440' },
			{ key: 'device.mobile', op: 'eq', value: true },
			{ key: 'browser.mobile', op: 'gte', value: 'true' },
			{ key: 'org', op: 'in', values: [12, true, 'a'] }
		]);
	});
});

describe('optionalNumber', () => {
	it('reads cleared and unparsable number inputs as unset', () => {
		expect(optionalNumber(undefined)).toBeUndefined();
		expect(optionalNumber(null)).toBeUndefined();
		expect(optionalNumber(Number.NaN)).toBeUndefined();
		expect(optionalNumber('')).toBeUndefined();
		expect(optionalNumber(' ')).toBeUndefined();
		expect(optionalNumber(0)).toBe(0);
		expect(optionalNumber(1.5)).toBe(1.5);
		expect(optionalNumber('25')).toBe(25);
	});

	it('edits the metrics block in the form and names it in the summary', () => {
		expect(fitsForm({ metrics: { enabled: false } })).toBe(true);
		expect(fitsForm({ metrics: { enabled: true, sampleRate: 0.1 } })).toBe(true);
		expect(jsonOnlyFields({ metrics: { sampleRate: 0.1, flushSeconds: 60 } })).toEqual([
			'metrics.flushSeconds'
		]);
		expect(summarizeSettings({ metrics: { enabled: false } })).toBe('metrics off');
		expect(summarizeSettings({ metrics: { sampleRate: 0.25 } })).toBe(
			'metrics from 25% of sessions'
		);
	});
});
