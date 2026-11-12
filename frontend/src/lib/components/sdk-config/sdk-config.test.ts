import { describe, expect, it } from 'vitest';
import {
	buildRuleConditions,
	conditionText,
	conditionValue,
	fitsForm,
	logLineTrigger,
	optionalNumber,
	summarizeConditions,
	summarizeSettings,
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

	it('only offers the form when it can show every field', () => {
		expect(
			fitsForm({ schemaVersion: 1, signals: { console: { mode: 'stream' } }, enabled: false })
		).toBe(true);
		expect(fitsForm({ signals: { console: { mode: 'stream', minSeverity: 'warn' } } })).toBe(false);
		expect(fitsForm({ triggers: [] })).toBe(false);
		expect(fitsForm({ sessions: { enabled: true, baselineSampleRate: 0.01 } })).toBe(true);
		expect(fitsForm({ sessions: { futureSetting: true } } as never)).toBe(false);
		expect(fitsForm({ replay: { mode: 'buffer', sessionSampleRate: 0.5 } })).toBe(false);
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
							{ key: 'signal', op: 'eq', value: 'console' },
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
});
