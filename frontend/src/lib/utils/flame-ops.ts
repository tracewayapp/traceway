import type { FlameGraphNode } from 'd3-flame-graph';

function deepClone(node: FlameGraphNode): FlameGraphNode {
	return {
		name: node.name,
		value: node.value,
		self: node.self ?? 0,
		children: (node.children ?? []).map(deepClone)
	};
}

function sortTree(node: FlameGraphNode): void {
	if (!node.children) return;
	node.children.sort((a, b) => b.value - a.value || a.name.localeCompare(b.name));
	for (const c of node.children) sortTree(c);
}

export function collapseRecursion(node: FlameGraphNode): FlameGraphNode {
	const out: FlameGraphNode = {
		name: node.name,
		value: node.value,
		self: node.self ?? 0,
		children: []
	};
	const byName = new Map<string, FlameGraphNode>();

	const addChild = (c: FlameGraphNode) => {
		if (c.name === out.name) {
			out.self = (out.self ?? 0) + (c.self ?? 0);
			for (const gc of c.children ?? []) addChild(gc);
			return;
		}
		const existing = byName.get(c.name);
		if (existing) {
			existing.value += c.value;
			existing.self = (existing.self ?? 0) + (c.self ?? 0);
			existing.children = [...(existing.children ?? []), ...(c.children ?? [])];
		} else {
			const copy: FlameGraphNode = {
				name: c.name,
				value: c.value,
				self: c.self ?? 0,
				children: [...(c.children ?? [])]
			};
			byName.set(c.name, copy);
			out.children!.push(copy);
		}
	};

	for (const c of node.children ?? []) addChild(c);
	out.children = out.children!.map(collapseRecursion);
	return out;
}

export interface Sandwich {
	fn: string;
	callers: FlameGraphNode;
	callees: FlameGraphNode;
	total: number;
}

function mergeInto(target: FlameGraphNode, src: FlameGraphNode): void {
	target.value += src.value;
	target.self = (target.self ?? 0) + (src.self ?? 0);
	const byName = new Map<string, FlameGraphNode>();
	for (const c of target.children ?? []) byName.set(c.name, c);
	for (const sc of src.children ?? []) {
		const existing = byName.get(sc.name);
		if (existing) {
			mergeInto(existing, sc);
		} else {
			const copy = deepClone(sc);
			byName.set(sc.name, copy);
			(target.children ??= []).push(copy);
		}
	}
}

export function buildSandwich(root: FlameGraphNode | null, fn: string): Sandwich | null {
	if (!root) return null;

	const calleeRoot: FlameGraphNode = { name: fn, value: 0, self: 0, children: [] };
	const callerRoot: FlameGraphNode = { name: fn, value: 0, self: 0, children: [] };

	const walk = (node: FlameGraphNode, ancestors: FlameGraphNode[]) => {
		if (node.name === fn) {
			mergeInto(calleeRoot, {
				name: fn,
				value: node.value,
				self: node.self ?? 0,
				children: node.children ?? []
			});

			let cursor = callerRoot;
			cursor.value += node.value;
			for (let i = ancestors.length - 1; i >= 0; i--) {
				const a = ancestors[i];
				let child = (cursor.children ?? []).find((c) => c.name === a.name);
				if (!child) {
					child = { name: a.name, value: 0, self: 0, children: [] };
					(cursor.children ??= []).push(child);
				}
				child.value += node.value;
				cursor = child;
			}
			return;
		}
		for (const c of node.children ?? []) walk(c, [...ancestors, node]);
	};

	walk(root, []);
	if (calleeRoot.value === 0) return null;

	sortTree(callerRoot);
	sortTree(calleeRoot);
	return { fn, callers: callerRoot, callees: calleeRoot, total: calleeRoot.value };
}
