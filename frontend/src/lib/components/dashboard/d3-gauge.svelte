<script lang="ts">
	import { arc } from 'd3-shape';
	import {
		activeThresholdColor,
		thresholdBandSegments,
		type ThresholdStep
	} from './gauge-thresholds';

	let {
		value,
		min = 0,
		max = 100,
		baseColor = 'green',
		thresholds = [],
		formatValue = (v: number) => v.toString()
	} = $props<{
		value: number;
		min?: number;
		max?: number;
		baseColor?: string;
		thresholds?: ThresholdStep[];
		formatValue?: (v: number) => string;
	}>();

	let containerRef = $state<HTMLDivElement | null>(null);
	let width = $state(0);
	let height = $state(0);

	$effect(() => {
		if (!containerRef) return;
		const observer = new ResizeObserver((entries) => {
			for (const entry of entries) {
				width = entry.contentRect.width;
				height = entry.contentRect.height;
			}
		});
		observer.observe(containerRef);
		return () => observer.disconnect();
	});

	const START_ANGLE = (-3 * Math.PI) / 4;
	const END_ANGLE = (3 * Math.PI) / 4;

	const safeMax = $derived(max > min ? max : min + 1);

	const radius = $derived(Math.max(0, Math.min((width - 16) / 2, (height - 34) / 1.71)));
	const contentHeight = $derived(radius * 1.71 + 34);
	const cx = $derived(width / 2);
	const cy = $derived(radius + 6 + Math.max(0, (height - contentHeight) / 2));

	const bandWidth = $derived(Math.max(3, radius * 0.06));
	const arcWidth = $derived(Math.max(8, radius * 0.17));
	const trackOuter = $derived(radius - bandWidth - 2);
	const trackInner = $derived(Math.max(0, trackOuter - arcWidth));

	function angleFor(v: number): number {
		const t = (Math.min(Math.max(v, min), safeMax) - min) / (safeMax - min);
		return START_ANGLE + t * (END_ANGLE - START_ANGLE);
	}

	const trackPath = $derived(
		arc()({
			innerRadius: trackInner,
			outerRadius: trackOuter,
			startAngle: START_ANGLE,
			endAngle: END_ANGLE
		}) ?? ''
	);

	const valuePath = $derived(
		arc()({
			innerRadius: trackInner,
			outerRadius: trackOuter,
			startAngle: START_ANGLE,
			endAngle: angleFor(value)
		}) ?? ''
	);

	const bandPaths = $derived(
		thresholdBandSegments(min, safeMax, baseColor, thresholds).map((seg) => ({
			color: seg.color,
			path:
				arc()({
					innerRadius: radius - bandWidth,
					outerRadius: radius,
					startAngle: angleFor(seg.from),
					endAngle: angleFor(seg.to)
				}) ?? ''
		}))
	);

	const valueColor = $derived(activeThresholdColor(value, baseColor, thresholds));
	const valueLabel = $derived(formatValue(value));

	const valueFontSize = $derived.by(() => {
		const fit = (trackInner * 1.8) / Math.max(valueLabel.length * 0.58, 1);
		return Math.max(12, Math.min(radius * 0.34, fit));
	});

	const endX = $derived(((trackInner + trackOuter) / 2) * Math.SQRT1_2);
	const endY = $derived(trackOuter * Math.SQRT1_2 + 18);
</script>

<div bind:this={containerRef} class="h-full w-full">
	{#if radius > 20}
		<svg {width} {height}>
			<g transform="translate({cx}, {cy})">
				{#each bandPaths as seg}
					<path d={seg.path} fill={seg.color} />
				{/each}
				<path d={trackPath} fill="var(--muted)" />
				<path d={valuePath} fill={valueColor} />
				<text
					text-anchor="middle"
					dominant-baseline="middle"
					font-size={valueFontSize}
					font-weight="700"
					fill={valueColor}
				>
					{valueLabel}
				</text>
				<text
					x={-endX}
					y={endY}
					text-anchor="middle"
					font-size="11"
					fill="var(--muted-foreground)"
				>
					{formatValue(min)}
				</text>
				<text
					x={endX}
					y={endY}
					text-anchor="middle"
					font-size="11"
					fill="var(--muted-foreground)"
				>
					{formatValue(max)}
				</text>
			</g>
		</svg>
	{/if}
</div>
