import { describe, expect, it } from 'vitest';
import { timestampNanos } from './timestamp-nanos';

describe('timestampNanos', () => {
	it('keeps every fractional digit a Go RFC3339Nano timestamp can carry', () => {
		const base = BigInt(Date.UTC(2026, 8, 17)) * 1_000_000n;
		expect(timestampNanos('2026-09-17T00:00:00Z')).toBe(base);
		expect(timestampNanos('2026-09-17T00:00:00.000000001Z')).toBe(base + 1n);
		expect(timestampNanos('2026-09-17T00:00:00.5Z')).toBe(base + 500_000_000n);
		expect(timestampNanos('2026-09-17T00:00:00.123456789Z')).toBe(base + 123_456_789n);
		expect(timestampNanos('2026-09-17T00:00:00.1234567891Z')).toBe(base + 123_456_789n);
	});

	it('applies zone offsets and treats unparseable input as zero', () => {
		expect(timestampNanos('2026-09-17T02:00:00.000000007+02:00')).toBe(
			timestampNanos('2026-09-17T00:00:00.000000007Z')
		);
		expect(timestampNanos('not a time')).toBe(0n);
		expect(timestampNanos('')).toBe(0n);
	});
});
