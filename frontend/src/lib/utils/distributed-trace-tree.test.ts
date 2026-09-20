import { expect, it } from 'vitest';
import { distributedTraceTree } from './distributed-trace-tree';
import type { DistributedTraceNode } from '$lib/types/distributed-trace';

function node(id: string, parent?: string, trace = 'trace'): DistributedTraceNode {
	return {
		projectId: `project-${id}`,
		projectName: id,
		traceType: 'endpoint',
		traceId: trace,
		spanId: id,
		spans: [],
		endpoint: {
			id,
			parentSpanId: parent,
			endpoint: id,
			recordedAt: '2026-09-17T00:00:00Z',
			duration: 100,
			statusCode: 200
		}
	};
}

it('connects authorized services through intermediate spans and collapses only descendants', () => {
	const parent = node('01');
	const child = node('03', '02');
	const independent = node('04');
	parent.spans = [
		{
			spanId: '02',
			parentSpanId: '01',
			traceId: 'trace',
			projectId: parent.projectId,
			name: 'client',
			startTime: '',
			recordedAt: '',
			duration: 10
		}
	];
	const rows = distributedTraceTree([child, independent, parent], new Set());
	expect(rows.map((r) => [r.node.endpoint?.id, r.depth, r.hasChildren])).toEqual([
		['01', 0, true],
		['03', 1, false],
		['04', 0, false]
	]);
	expect(
		distributedTraceTree([child, independent, parent], new Set([rows[0].key])).map(
			(r) => r.node.endpoint?.id
		)
	).toEqual(['01', '04']);
});

it('does not manufacture relationships from ordering or custom grouping', () => {
	const rows = distributedTraceTree([node('01'), node('02', '01', 'other-trace')], new Set());
	expect(rows.every((r) => r.depth === 0 && !r.hasChildren)).toBe(true);
});

it('keeps duplicate source identities in different projects visible without guessing a parent', () => {
	const first = node('01');
	const second = { ...node('01'), projectId: 'another-project' };
	const child = node('02', '01');
	const rows = distributedTraceTree([first, second, child], new Set());
	expect(rows).toHaveLength(3);
	expect(new Set(rows.map((r) => r.key)).size).toBe(3);
	expect(rows.every((r) => r.depth === 0)).toBe(true);
});

it('nests across a hop that belongs to no entity when the server names the ancestor', () => {
	const gateway = node('0000000000000001');
	// The parent is a gRPC client span in a project with no promoted entity, so no loaded span leads back to gateway.
	const warehouse = node('0000000000000005', '0000000000000004');
	expect(distributedTraceTree([gateway, warehouse], new Set()).map((row) => row.depth)).toEqual([
		0, 0
	]);
	const linked = { ...warehouse, parentEntitySpanId: '0000000000000001' };
	const rows = distributedTraceTree([gateway, linked], new Set());
	expect(rows.map((row) => [row.node.projectName, row.depth])).toEqual([
		['0000000000000001', 0],
		['0000000000000005', 1]
	]);
});
