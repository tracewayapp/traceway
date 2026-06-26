declare module 'd3-flame-graph' {
	export interface FlameGraphNode {
		name: string;
		value: number;
		self?: number;
		delta?: number;
		children?: FlameGraphNode[];
	}

	export interface FlameGraphHierarchyNode {
		data: FlameGraphNode;
		value: number;
		parent: FlameGraphHierarchyNode | null;
	}

	export interface FlameGraph {
		(selection: unknown): void;
		width(value: number): FlameGraph;
		height(value: number): FlameGraph;
		cellHeight(value: number): FlameGraph;
		minFrameSize(value: number): FlameGraph;
		transitionDuration(value: number): FlameGraph;
		inverted(value: boolean): FlameGraph;
		sort(value: boolean): FlameGraph;
		selfValue(value: boolean): FlameGraph;
		setColorMapper(
			fn: (node: FlameGraphHierarchyNode, originalColor: string) => string
		): FlameGraph;
		label(fn: (node: FlameGraphHierarchyNode) => string): FlameGraph;
		onClick(fn: (node: FlameGraphHierarchyNode) => void): FlameGraph;
		resetZoom(): void;
		search(term: string): void;
		clear(): void;
		update(root?: FlameGraphNode): void;
		destroy(): void;
	}

	export default function flamegraph(): FlameGraph;
	export function colorMapper(node: unknown): string;
	export function tooltip(): unknown;
}
