import { buildRuleConditions, conditionText } from './sdk-config';
import { describe, expect, it } from 'vitest';
import {
	fitsForm,
	logLineTrigger,
	summarizeConditions,
	summarizeSettings,
	uniqueTriggerId
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
});
