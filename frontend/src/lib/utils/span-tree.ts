import type { Span } from '$lib/types/spans';
import { timestampNanos } from './timestamp-nanos';

export type SpanTreeRow = {
	key: string;
	span: Span;
	depth: number;
	hasChildren: boolean;
	missingParent: boolean;
	descendants: number;
	errorBelow: boolean;
};

export type SpanTree = {
	byId: Map<string, Span>;
	childrenById: Map<string, string[]>;
	roots: string[];
	missing: Set<string>;
	descendants: Map<string, number>;
	errorBelow: Set<string>;
};

const SPAN_STATUS_ERROR = 2;

// A trace this large opens with its widest nodes folded, the way Datadog folds hidden spans into a numbered pill.
export const COLLAPSE_MIN_SPANS = 500;
export const COLLAPSE_MIN_CHILDREN = 50;

// A span normally links only inside its own project, which keeps the same span exported to two projects apart. A whole
// trace read across projects is the exception: there a child in one project hangs under its parent in another.
function keyScope(span: Span, acrossProjects: boolean): string {
	return acrossProjects ? 'trace' : span.projectId;
}

export function spanTreeKey(span: Span, acrossProjects = false): string {
	return `${keyScope(span, acrossProjects)}:${span.traceId}:${span.spanId}`;
}

export function buildSpanTree(
	spans: Span[],
	rootSpanId: string | undefined,
	acrossProjects = false
): SpanTree {
	const byId = new Map<string, Span>();
	for (const span of spans) {
		const key = spanTreeKey(span, acrossProjects);
		const old = byId.get(key);
		if (!old || span.duration > old.duration) byId.set(key, span);
	}
	const parentById = new Map<string, string>();
	const missing = new Set<string>();
	for (const [key, span] of byId) {
		if (!span.parentSpanId) continue;
		const parentKey = `${keyScope(span, acrossProjects)}:${span.traceId}:${span.parentSpanId}`;
		if (parentKey !== key && byId.has(parentKey)) {
			parentById.set(key, parentKey);
		} else if (span.parentSpanId !== rootSpanId) {
			missing.add(key);
		}
	}

	// Corrupt cycles become roots; iterative traversal also handles very deep traces.
	const checked = new Set<string>();
	for (const key of byId.keys()) {
		const path = new Set<string>();
		let current: string | undefined = key;
		while (current && !checked.has(current)) {
			if (path.has(current)) {
				parentById.delete(current);
				missing.add(current);
				break;
			}
			path.add(current);
			current = parentById.get(current);
		}
		for (const visited of path) checked.add(visited);
	}

	const childrenById = new Map<string, string[]>();
	const roots: string[] = [];
	for (const key of byId.keys()) {
		const parent = parentById.get(key);
		if (!parent) roots.push(key);
		else {
			const children = childrenById.get(parent) ?? [];
			children.push(key);
			childrenById.set(parent, children);
		}
	}
	const startNanos = new Map<string, bigint>();
	for (const [key, span] of byId) startNanos.set(key, timestampNanos(span.startTime));
	const compare = (a: string, b: string) => {
		const difference = startNanos.get(a)! - startNanos.get(b)!;
		return difference < 0n ? -1 : difference > 0n ? 1 : a.localeCompare(b);
	};
	roots.sort(compare);
	for (const children of childrenById.values()) children.sort(compare);

	// Children before parents, without recursion: a trace can be thousands of spans deep.
	const descendants = new Map<string, number>();
	const errorBelow = new Set<string>();
	const order: string[] = [];
	const stack = [...roots];
	while (stack.length) {
		const key = stack.pop()!;
		order.push(key);
		for (const child of childrenById.get(key) ?? []) stack.push(child);
	}
	for (let i = order.length - 1; i >= 0; i--) {
		const key = order[i];
		let count = 0;
		for (const child of childrenById.get(key) ?? []) {
			count += 1 + (descendants.get(child) ?? 0);
			if (errorBelow.has(child) || byId.get(child)!.statusCode === SPAN_STATUS_ERROR) {
				errorBelow.add(key);
			}
		}
		descendants.set(key, count);
	}

	return { byId, childrenById, roots, missing, descendants, errorBelow };
}

export function initiallyCollapsed(tree: SpanTree): Set<string> {
	const collapsed = new Set<string>();
	if (tree.byId.size <= COLLAPSE_MIN_SPANS) return collapsed;
	for (const [key, children] of tree.childrenById) {
		if (children.length > COLLAPSE_MIN_CHILDREN) collapsed.add(key);
	}
	return collapsed;
}

export function flattenBuiltTree(tree: SpanTree, collapsed: Set<string>): SpanTreeRow[] {
	const rows: SpanTreeRow[] = [];
	const pending = tree.roots.map((key) => ({ key, depth: 0 })).reverse();
	while (pending.length) {
		const { key, depth } = pending.pop()!;
		const children = tree.childrenById.get(key) ?? [];
		rows.push({
			key,
			span: tree.byId.get(key)!,
			depth,
			hasChildren: children.length > 0,
			missingParent: tree.missing.has(key),
			descendants: tree.descendants.get(key) ?? 0,
			errorBelow: tree.errorBelow.has(key)
		});
		if (collapsed.has(key)) continue;
		for (let i = children.length - 1; i >= 0; i--) {
			pending.push({ key: children[i], depth: depth + 1 });
		}
	}
	return rows;
}

export function flattenSpanTree(
	spans: Span[],
	rootSpanId: string | undefined,
	collapsed: Set<string>
): SpanTreeRow[] {
	return flattenBuiltTree(buildSpanTree(spans, rootSpanId), collapsed);
}
