import { describe, expect, it } from 'vitest';
import { metricUnitLabel } from './metric-format';

describe('metricUnitLabel', () => {
	it('names OpenTelemetry units in words', () => {
		expect(metricUnitLabel('By')).toBe('bytes');
		expect(metricUnitLabel('By/s')).toBe('bytes/s');
		expect(metricUnitLabel('1')).toBe('%');
		expect(metricUnitLabel('{requests}')).toBe('requests');
	});

	it('keeps units that already read well', () => {
		expect(metricUnitLabel('ms')).toBe('ms');
		expect(metricUnitLabel('MB')).toBe('MB');
		expect(metricUnitLabel('%')).toBe('%');
	});
});
