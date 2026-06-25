import type { FlameGraphNode } from 'd3-flame-graph';

function buildDiffNode(
	curNode: FlameGraphNode,
	baseNode: FlameGraphNode | undefined,
	curTotal: number,
	baseTotal: number
): FlameGraphNode {
	const curFrac = curTotal > 0 ? curNode.value / curTotal : 0;
	const baseFrac = baseNode && baseTotal > 0 ? baseNode.value / baseTotal : 0;

	const baseChildren = new Map<string, FlameGraphNode>();
	for (const c of baseNode?.children ?? []) baseChildren.set(c.name, c);

	return {
		name: curNode.name,
		value: curNode.value,
		self: curNode.self,
		delta: curFrac - baseFrac,
		children: (curNode.children ?? []).map((c) =>
			buildDiffNode(c, baseChildren.get(c.name), curTotal, baseTotal)
		)
	};
}

export function mergeFlameTrees(
	base: FlameGraphNode | null,
	cur: FlameGraphNode | null
): FlameGraphNode | null {
	if (!cur) return null;
	return buildDiffNode(cur, base ?? undefined, cur.value || 0, base?.value || 0);
}
