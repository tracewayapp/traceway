import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/svelte';

const flamegraphSpy = vi.hoisted(() => vi.fn());
const datumSpy = vi.hoisted(() => vi.fn());

vi.mock('d3-flame-graph', () => {
	const chart = new Proxy(() => chart, { get: () => () => chart }) as never;
	return { default: () => (flamegraphSpy(), chart), colorMapper: () => '', tooltip: () => ({}) };
});

vi.mock('d3-selection', () => {
	const sel: Record<string, unknown> = {};
	sel.datum = (d: unknown) => (datumSpy(d), sel);
	sel.call = () => sel;
	sel.selectAll = () => sel;
	sel.remove = () => sel;
	sel.html = () => sel;
	sel.node = () => null;
	return { select: () => sel };
});

import FlameGraph, { flameTooltipLabel } from './flame-graph.svelte';

const NS = 'nanoseconds';

const tree = {
	name: 'root',
	value: 100,
	self: 0,
	children: [{ name: 'main.work', value: 60, self: 60, children: [] }]
};

describe('flameTooltipLabel', () => {
	it('formats name, value, % of total and % of parent', () => {
		expect(flameTooltipLabel('main.work', 1_500_000, 0, 3_000_000, 4_000_000, NS)).toBe(
			'main.work · 1.5 ms (37.5% of total) · 50% of parent'
		);
	});

	it('appends self time when a frame has non-leaf self', () => {
		expect(flameTooltipLabel('main.serve', 100, 40, 200, 200, NS)).toBe(
			'main.serve · 100 ns (50% of total) · 50% of parent · self 40 ns'
		);
	});

	it('omits the self line for a leaf where self equals value', () => {
		expect(flameTooltipLabel('main.leaf', 60, 60, 100, 100, NS)).toBe(
			'main.leaf · 60 ns (60% of total) · 60% of parent'
		);
	});

	it('renders 0% and no parent/self when the total and parent are zero', () => {
		expect(flameTooltipLabel('main.work', 10, 0, 0, 0, NS)).toBe(
			'main.work · 10 ns (0% of total)'
		);
	});
});

describe('FlameGraph component', () => {
	beforeEach(() => {
		flamegraphSpy.mockClear();
		datumSpy.mockClear();
	});

	it('shows an empty state and does not render a chart when data is null', () => {
		const { getByText } = render(FlameGraph, { props: { data: null, unit: NS } });
		expect(getByText(/no flame graph data/i)).toBeTruthy();
		expect(flamegraphSpy).not.toHaveBeenCalled();
	});

	it('renders the d3 flame graph with the provided tree', () => {
		render(FlameGraph, { props: { data: tree, unit: NS } });
		expect(flamegraphSpy).toHaveBeenCalled();
		expect(datumSpy).toHaveBeenCalledWith(tree);
	});

	it('re-renders when the data prop changes', async () => {
		const { rerender } = render(FlameGraph, { props: { data: tree, unit: NS } });
		datumSpy.mockClear();
		const next = { name: 'root', value: 50, self: 0, children: [] };
		await rerender({ data: next, unit: NS });
		expect(datumSpy).toHaveBeenCalledWith(next);
	});
});
