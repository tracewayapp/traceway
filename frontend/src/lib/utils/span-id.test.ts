import { describe, expect, it } from 'vitest';
import { logTraceId } from './span-id';

const traceId = '8462dd06-1570-32df-3c6c-90ac5ff74ed7';
const traceHex = '8462dd06157032df3c6c90ac5ff74ed7';
const occurrenceId = '00000000-0000-0000-1985-a7abed0024db';
const groupId = 'a5100000-0000-4000-8000-000000000001';

describe('logTraceId', () => {
	it('correlates existing non-root occurrences without requiring a stored upstream root', () => {
		expect(logTraceId({ id: occurrenceId, distributedTraceId: traceId })).toBe(traceHex);
	});

	it('keeps native trace IDs independent of distributed grouping', () => {
		expect(logTraceId({ id: traceId, distributedTraceId: groupId })).toBe(traceHex);
		expect(logTraceId({ id: traceId, attributes: null })).toBe(traceHex);
	});

	it('uses the original OTel ID even when distributed grouping is overridden', () => {
		for (const id of [occurrenceId, traceId]) {
			expect(
				logTraceId({
					id,
					distributedTraceId: groupId,
					attributes: { 'traceway.otel.trace_id': traceHex.toUpperCase() }
				})
			).toBe(traceHex);
		}
	});
});
