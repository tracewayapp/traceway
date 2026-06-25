import { describe, it, expect } from 'vitest';
import { formatValue, humanizeType, defaultProfileType } from './profile-format';

describe('formatValue', () => {
	it('formats time units, normalizing to a readable scale', () => {
		expect(formatValue('nanoseconds', 500)).toBe('500 ns');
		expect(formatValue('nanoseconds', 1_500)).toBe('1.5 µs');
		expect(formatValue('milliseconds', 1_500)).toBe('1.5 s');
		expect(formatValue('microseconds', 2_000)).toBe('2 ms');
		expect(formatValue('seconds', 3)).toBe('3 s');
	});

	it('formats byte units', () => {
		expect(formatValue('bytes', 512)).toBe('512 B');
		expect(formatValue('bytes', 1024)).toBe('1 KB');
		expect(formatValue('bytes', 1_048_576)).toBe('1 MB');
	});

	it('formats count-like units as localized integers', () => {
		expect(formatValue('count', 1500)).toBe((1500).toLocaleString());
		expect(formatValue('samples', 42)).toBe('42');
		expect(formatValue('', 7)).toBe('7');
	});

	it('falls back to a localized number plus the raw unit for unknown units', () => {
		expect(formatValue('widgets', 1234)).toBe(`${(1234).toLocaleString()} widgets`);
	});
});

describe('humanizeType', () => {
	it('uses short friendly labels for the well-known pprof names', () => {
		expect(humanizeType('cpu')).toBe('CPU');
		expect(humanizeType('inuse_space')).toBe('In-Use');
		expect(humanizeType('goroutine')).toBe('Goroutines');
		expect(humanizeType('lock')).toBe('Lock');
	});

	it('title-cases arbitrary cross-language type names', () => {
		expect(humanizeType('async_profiler_wall')).toBe('Async Profiler Wall');
		expect(humanizeType('jvm:heap_live')).toBe('Heap Live');
	});
});

describe('defaultProfileType', () => {
	it('prefers cpu when present', () => {
		expect(defaultProfileType(['inuse_space', 'cpu', 'lock'])).toBe('cpu');
	});

	it('falls back to the first available type', () => {
		expect(defaultProfileType(['lock', 'wall'])).toBe('lock');
	});

	it('returns empty string when nothing is available', () => {
		expect(defaultProfileType([])).toBe('');
	});
});
