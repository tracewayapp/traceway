<script module lang="ts">
	import { formatValue } from '$lib/utils/profile-format';
	import type { FlameGraphNode } from 'd3-flame-graph';

	function trimPct(pct: number): string {
		return Number(pct.toFixed(1)).toString();
	}

	export function flameTooltipLabel(
		name: string,
		value: number,
		self: number,
		parentValue: number,
		total: number,
		unit: string
	): string {
		const pctTotal = total > 0 ? (value / total) * 100 : 0;
		const parts = [name, `${formatValue(unit, value)} (${trimPct(pctTotal)}% of total)`];
		if (parentValue > 0 && parentValue !== value) {
			parts.push(`${trimPct((value / parentValue) * 100)}% of parent`);
		}
		if (self > 0 && self !== value) {
			parts.push(`self ${formatValue(unit, self)}`);
		}
		return parts.join(' · ');
	}

	export function diffColor(delta: number): string {
		const mag = Math.min(1, Math.abs(delta));
		if (delta > 0.001) return `hsl(8, ${Math.round(45 + mag * 45)}%, ${Math.round(68 - mag * 22)}%)`;
		if (delta < -0.001)
			return `hsl(140, ${Math.round(40 + mag * 45)}%, ${Math.round(66 - mag * 20)}%)`;
		return 'hsl(220, 6%, 78%)';
	}

	export function flameDepth(node: FlameGraphNode): number {
		if (!node.children || node.children.length === 0) return 0;
		let max = 0;
		for (const child of node.children) {
			const d = flameDepth(child);
			if (d > max) max = d;
		}
		return max + 1;
	}
</script>

<script lang="ts">
	import flamegraph, { type FlameGraph } from 'd3-flame-graph';
	import { select } from 'd3-selection';
	import { untrack } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { RotateCcw, FlipVertical2 } from '@lucide/svelte';

	interface Props {
		data: FlameGraphNode | null;
		unit: string;
		diff?: boolean;
		controls?: boolean;
		searchTerm?: string;
		inverted?: boolean;
		onFrameSelect?: (name: string) => void;
	}

	let {
		data,
		unit,
		diff = false,
		controls = true,
		searchTerm = $bindable(''),
		inverted: invertedProp = false,
		onFrameSelect
	}: Props = $props();

	const CELL = 20;

	let container = $state<HTMLDivElement>();
	let width = $state(0);
	let chart: FlameGraph | null = null;

	let inverted = $state(invertedProp);

	const contentHeight = $derived(data ? (flameDepth(data) + 1) * CELL : 0);
	const svgShift = $derived(inverted ? 0 : -2 * CELL);

	$effect(() => {
		const el = container;
		if (!el || typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver((entries) => {
			width = entries[0].contentRect.width;
		});
		observer.observe(el);
		return () => observer.disconnect();
	});

	$effect(() => {
		const el = container;
		if (!el || !data) return;

		const total = data.value || 0;
		const currentUnit = unit;

		const instance = flamegraph()
			.width(el.clientWidth || 960)
			.cellHeight(CELL)
			.minFrameSize(1)
			.transitionDuration(200)
			.sort(true)
			.selfValue(false)
			.inverted(inverted)
			.label((d) =>
				flameTooltipLabel(
					d.data.name,
					d.data.value,
					d.data.self ?? 0,
					d.parent?.data.value ?? 0,
					total,
					currentUnit
				)
			);

		if (diff) {
			instance.setColorMapper((node) => diffColor(node.data.delta ?? 0));
		}

		if (onFrameSelect) {
			instance.onClick((node) => onFrameSelect(node.data.name));
		}

		select(el).datum(data).call(instance);
		chart = instance;

		const active = untrack(() => searchTerm).trim();
		if (active) instance.search(active);

		return () => {
			try {
				instance.destroy();
			} catch {
				select(el).selectAll('*').remove();
			}
			select(el).selectAll('*').remove();
			chart = null;
		};
	});

	$effect(() => {
		const w = width;
		if (!chart || !w) return;
		try {
			chart.width(w);
			chart.update();
		} catch {
			/* resize is best-effort */
		}
	});

	$effect(() => {
		const term = searchTerm.trim();
		void data;
		if (!chart) return;
		if (term) chart.search(term);
		else chart.clear();
	});

	function resetZoom() {
		chart?.resetZoom();
	}
</script>

{#if !data}
	<div
		class="flex h-48 items-center justify-center rounded-md border text-sm text-muted-foreground"
	>
		No flame graph data for this range.
	</div>
{:else}
	<div class="mb-3 flex flex-wrap items-center gap-2">
		{#if controls}
			<Input
				type="text"
				placeholder="Search frames…"
				bind:value={searchTerm}
				data-testid="flame-search"
				class="h-8 w-56"
			/>
		{/if}
		<div class="flex-1"></div>
		<Button
			variant="outline"
			size="sm"
			onclick={resetZoom}
			data-testid="flame-reset"
			title="Reset zoom"
		>
			<RotateCcw class="h-4 w-4" />
			Reset
		</Button>
		{#if controls}
			<Button
				variant={inverted ? 'secondary' : 'outline'}
				size="sm"
				aria-pressed={inverted}
				onclick={() => (inverted = !inverted)}
				data-testid="flame-icicle"
				title="Toggle icicle (top-down) view"
			>
				<FlipVertical2 class="h-4 w-4" />
				Icicle
			</Button>
		{/if}
	</div>
	<div
		bind:this={container}
		class="flame-graph w-full overflow-hidden"
		style="height: {contentHeight}px; --flame-svg-shift: {svgShift}px"
	></div>
{/if}

<style>
	:global(.flame-graph svg) {
		display: block;
		margin-top: var(--flame-svg-shift, 0);
	}

	:global(.flame-graph .d3-flame-graph rect) {
		stroke: var(--background);
		stroke-width: 0.5;
		fill-opacity: 0.85;
	}

	:global(.flame-graph .d3-flame-graph rect:hover) {
		stroke: var(--ring);
		stroke-width: 1;
		cursor: pointer;
	}

	:global(.flame-graph .d3-flame-graph-label) {
		white-space: nowrap;
		text-overflow: ellipsis;
		overflow: hidden;
		font-size: 12px;
		font-family: inherit;
		margin-left: 4px;
		margin-right: 4px;
		line-height: 1.5;
		font-weight: 400;
		color: #18181b;
		text-align: left;
	}

	:global(.dark .flame-graph .d3-flame-graph-label) {
		color: #fff;
	}

	:global(.flame-graph .d3-flame-graph .fade) {
		opacity: 0.5 !important;
	}

	:global(.d3-flame-graph-tip) {
		background: var(--popover);
		color: var(--popover-foreground);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 10px;
		min-width: 250px;
		text-align: left;
		font-family: inherit;
		font-size: 12px;
		z-index: 10;
	}
</style>
