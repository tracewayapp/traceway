import { describe, expect, it } from 'vitest';
import {
	buildSpanTree,
	flattenBuiltTree,
	flattenSpanTree,
	initiallyCollapsed,
	spanTreeKey
} from './span-tree';
import type { Span } from '$lib/types/spans';

function span(id: string, parentSpanId?: string, trace = 'trace-a'): Span {
	return {
		spanId: id,
		traceId: trace,
		projectId: 'project',
		name: id,
		parentSpanId,
		startTime: '2026-09-17T00:00:00Z',
		duration: 100,
		recordedAt: ''
	};
}

describe('span graph', () => {
	it('nests by protocol identity and hides only actual descendants', () => {
		const parent = span('0000000000000001');
		const child = span('0000000000000002', parent.spanId);
		const grandchild = span('0000000000000003', child.spanId);
		const sibling = span('0000000000000004');
		const input = [grandchild, sibling, child, parent];
		expect(
			flattenSpanTree(input, undefined, new Set()).map((r) => [
				r.span.spanId,
				r.depth,
				r.hasChildren
			])
		).toEqual([
			[parent.spanId, 0, true],
			[child.spanId, 1, true],
			[grandchild.spanId, 2, false],
			[sibling.spanId, 0, false]
		]);
		expect(
			flattenSpanTree(input, undefined, new Set([spanTreeKey(parent)])).map((r) => r.span.spanId)
		).toEqual([parent.spanId, sibling.spanId]);
	});
	it('keeps missing parents visible and does not join different traces or projects', () => {
		const parent = span('01');
		const orphan = span('02', '01', 'trace-b');
		const otherProject = { ...span('03', '01'), projectId: 'other' };
		const rows = flattenSpanTree([parent, orphan, otherProject], undefined, new Set());
		expect(rows.every((r) => r.depth === 0 && !r.hasChildren)).toBe(true);
		expect(rows.filter((r) => r.missingParent)).toHaveLength(2);
	});
	it('deduplicates retries and breaks cycles without phantom children', () => {
		const a = span('01', '02');
		const b = span('02', '01');
		const self = span('03', '03');
		const rows = flattenSpanTree([a, b, a, self], undefined, new Set());
		expect(rows).toHaveLength(3);
		expect(rows.filter((r) => r.hasChildren)).toHaveLength(1);
		expect(flattenSpanTree([a, b], undefined, new Set([rows[0].key]))).toHaveLength(1);
	});
	it('orders siblings by exact start time and breaks ties by source ID', () => {
		const at = (id: string, startTime: string) => ({ ...span(id), startTime });
		const ordered = [
			at('000000000000000a', '2026-09-17T00:00:00.000000001Z'),
			at('000000000000000b', '2026-09-17T00:00:00.000000002Z'),
			at('000000000000000c', '2026-09-17T00:00:00.0001Z'),
			at('000000000000000d', '2026-09-17T00:00:00.0002Z'),
			at('000000000000000e', '2026-09-17T00:00:01Z'),
			at('000000000000000f', '2026-09-17T00:00:01Z')
		];
		const expected = ordered.map((s) => s.spanId);
		for (const input of [
			[...ordered].reverse(),
			[ordered[3], ordered[0], ordered[5], ordered[2], ordered[4], ordered[1]]
		]) {
			expect(flattenSpanTree(input, undefined, new Set()).map((r) => r.span.spanId)).toEqual(
				expected
			);
		}
	});

	it('handles deep traces without recursion', () => {
		const spans = Array.from({ length: 15000 }, (_, i) =>
			span(String(i + 1), i ? String(i) : undefined)
		);
		expect(flattenSpanTree(spans, undefined, new Set())).toHaveLength(15000);
	});
	it('nests spans of the native protocol by their 32 character ids', () => {
		const a = span('11111111111111111111111111111111');
		const b = span('22222222222222222222222222222222', a.spanId);
		expect(flattenSpanTree([b, a], undefined, new Set()).map((r) => r.depth)).toEqual([0, 1]);
	});
	it('does not mark the spans directly under the root span as orphans', () => {
		const child = span('02', '01');
		const orphan = span('03', '09');
		const rows = flattenSpanTree([child, orphan], '01', new Set());
		expect(rows.map((r) => [r.span.spanId, r.missingParent])).toEqual([
			['02', false],
			['03', true]
		]);
	});
});

describe('large traces', () => {
	const make = (id: number, parent?: number, statusCode?: number): Span => ({
		spanId: id.toString(16).padStart(16, '0'),
		parentSpanId: parent === undefined ? undefined : parent.toString(16).padStart(16, '0'),
		traceId: 'trace',
		projectId: 'project',
		name: `span ${id}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000,
		statusCode
	});

	it('counts what is below each node and knows where an error hides', () => {
		const tree = buildSpanTree(
			[make(1), make(2, 1), make(3, 2, 2), make(4, 1), make(5)],
			undefined
		);
		const rows = flattenBuiltTree(tree, new Set());
		expect(rows.map((row) => [row.span.name, row.descendants, row.errorBelow])).toEqual([
			['span 1', 3, true],
			['span 2', 1, true],
			['span 3', 0, false],
			['span 4', 0, false],
			['span 5', 0, false]
		]);
	});

	it('leave an ordinary trace fully open', () => {
		const spans = [make(1), ...Array.from({ length: 300 }, (_, i) => make(i + 2, 1))];
		expect(initiallyCollapsed(buildSpanTree(spans, undefined)).size).toBe(0);
	});

	it('open with only their widest nodes folded', () => {
		const wide = Array.from({ length: 600 }, (_, i) => make(i + 100, 2));
		const spans = [make(1), make(2, 1), make(3, 1), make(4, 3), ...wide];
		const tree = buildSpanTree(spans, undefined);
		const folded = initiallyCollapsed(tree);
		expect([...folded]).toEqual([spanTreeKey(spans[1])]);
		expect(flattenBuiltTree(tree, folded).map((row) => row.span.name)).toEqual([
			'span 1',
			'span 2',
			'span 3',
			'span 4'
		]);
	});
});

describe('a trace read across projects', () => {
	const hop = (projectId: string, spanId: string, parentSpanId?: string): Span => ({
		spanId,
		parentSpanId,
		traceId: 'trace',
		projectId,
		name: `${projectId} ${spanId}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000
	});
	const spans = [hop('gateway', '01'), hop('payments', '02', '01'), hop('worker', '03', '02')];

	it('hangs a child under its parent in another project', () => {
		const rows = flattenBuiltTree(buildSpanTree(spans, undefined, true), new Set());
		expect(rows.map((row) => [row.span.name, row.depth, row.missingParent])).toEqual([
			['gateway 01', 0, false],
			['payments 02', 1, false],
			['worker 03', 2, false]
		]);
	});

	it('keeps projects apart everywhere else', () => {
		const rows = flattenBuiltTree(buildSpanTree(spans, undefined), new Set());
		expect(rows.map((row) => [row.depth, row.missingParent])).toEqual([
			[0, false],
			[0, true],
			[0, true]
		]);
	});
});
