import type { DistributedTraceNode } from '$lib/types/distributed-trace';
import type { Span } from '$lib/types/spans';
import { flattenSpanTree, spanTreeKey } from './span-tree';

const wireKey = (trace: string, span: string) => `${trace}:${span}`;

export function distributedTraceTree(nodes: DistributedTraceNode[], collapsed: Set<string>) {
	const parents = new Map<string, Set<string>>();
	const addParent = (child: string, parent: string) => {
		const candidates = parents.get(child) ?? new Set<string>();
		candidates.add(parent);
		parents.set(child, candidates);
	};
	const parentOf = (key: string) => {
		const candidates = parents.get(key);
		return candidates?.size === 1 ? candidates.values().next().value : undefined;
	};
	// The server walks the whole trace across projects and names the nearest promoted entity above each one. That beats
	// the edges loaded here, which stop at any hop that belongs to no entity's subtree.
	const ancestors = new Map<string, string>();
	const roots = new Map<string, { projectId: string; span: Span }[]>();
	const nodesByKey = new Map<string, DistributedTraceNode>();
	const spans: Span[] = [];

	for (const node of nodes) {
		for (const span of node.spans) {
			if (span.parentSpanId) {
				addParent(wireKey(span.traceId, span.spanId), wireKey(span.traceId, span.parentSpanId));
			}
		}
		const entity = node.endpoint ?? node.task ?? node.aiTrace;
		const span: Span = {
			// Virtual tree identities keep duplicated exports in different projects distinct.
			projectId: 'distributed',
			traceId: 'distributed',
			spanId: `${node.projectId}:${node.traceType}:${node.traceId}:${node.spanId}:${entity?.id ?? `${node.exception?.exceptionHash}:${node.exception?.recordedAt}`}`,
			name: '',
			startTime: entity?.recordedAt ?? node.exception?.recordedAt ?? '',
			recordedAt: entity?.recordedAt ?? '',
			duration: entity?.duration ?? 0
		};
		if (entity && node.traceId && node.spanId) {
			const key = wireKey(node.traceId, node.spanId);
			const candidates = roots.get(key) ?? [];
			candidates.push({ projectId: node.projectId, span });
			roots.set(key, candidates);
			if (entity.parentSpanId) addParent(key, wireKey(node.traceId, entity.parentSpanId));
			if (node.parentEntitySpanId) {
				ancestors.set(key, wireKey(node.traceId, node.parentEntitySpanId));
			}
		}
		spans.push(span);
		nodesByKey.set(spanTreeKey(span), node);
	}

	for (const [key, entries] of roots) {
		for (const { projectId, span } of entries) {
			const visited = new Set([key]);
			let parent = ancestors.get(key) ?? parentOf(key);
			while (parent && !visited.has(parent)) {
				visited.add(parent);
				const candidates = roots.get(parent) ?? [];
				const local = candidates.filter((candidate) => candidate.projectId === projectId);
				const eligible = local.length ? local : candidates;
				if (eligible.length === 1) {
					span.parentSpanId = eligible[0].span.spanId;
					break;
				}
				if (eligible.length > 1) break;
				parent = parentOf(parent);
			}
		}
	}
	return flattenSpanTree(spans, undefined, collapsed).map((row) => ({
		...row,
		node: nodesByKey.get(row.key)!
	}));
}
