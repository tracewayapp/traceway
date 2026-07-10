<script lang="ts">
	import { scaleUtc, scaleLinear } from 'd3-scale';
	import { line } from 'd3-shape';
	import { min, max } from 'd3-array';
	import type { MetricTrendPoint } from '$lib/types/dashboard';

	let {
		data = [],
		color = '#3b82f6',
		height = 40
	} = $props<{
		data: MetricTrendPoint[];
		color?: string;
		height?: number;
	}>();

	let containerRef = $state<HTMLDivElement | null>(null);
	let width = $state(120);

	$effect(() => {
		if (!containerRef) return;

		const observer = new ResizeObserver((entries) => {
			for (const entry of entries) {
				width = entry.contentRect.width;
			}
		});

		observer.observe(containerRef);
		return () => observer.disconnect();
	});

	const path = $derived.by(() => {
		if (data.length < 2 || width <= 0) return '';

		const xScale = scaleUtc()
			.domain([
				min(data, (d: MetricTrendPoint) => d.timestamp) ?? new Date(),
				max(data, (d: MetricTrendPoint) => d.timestamp) ?? new Date()
			])
			.range([1, width - 1]);

		const yMin = min(data, (d: MetricTrendPoint) => d.value) ?? 0;
		const yMax = max(data, (d: MetricTrendPoint) => d.value) ?? 1;
		// Flat series would collapse to a zero-height domain; pad so the line stays visible
		const pad = yMax === yMin ? Math.abs(yMax) * 0.1 || 1 : 0;
		const yScale = scaleLinear()
			.domain([yMin - pad, yMax + pad])
			.range([height - 2, 2]);

		const lineGen = line<MetricTrendPoint>()
			.x((d) => xScale(d.timestamp))
			.y((d) => yScale(d.value));

		return lineGen(data) || '';
	});
</script>

<div bind:this={containerRef} class="w-full" style="height: {height}px;">
	{#if path}
		<svg {width} {height}>
			<path d={path} fill="none" stroke={color} stroke-width="1.5" stroke-opacity="0.8" />
		</svg>
	{/if}
</div>
