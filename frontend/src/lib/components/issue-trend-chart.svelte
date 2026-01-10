<script lang="ts">
	import * as Tooltip from "$lib/components/ui/tooltip";

	type TrendPoint = { timestamp: string; count: number };

	let { trend = [] }: {
		trend: TrendPoint[];
	} = $props();

	// Always 24 hours
	const HOURS = 24;
	const SCALE_MAX = 10; // Visual scale max - bars are relative to this

	// Build 24-hour data array, filling gaps with 0
	const hourlyData = $derived(() => {
		const now = new Date();
		const result: { hour: Date; count: number }[] = [];

		// Create a map of hour -> count from trend data
		const trendMap = new Map<string, number>();
		for (const point of trend) {
			const date = new Date(point.timestamp);
			const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}-${date.getHours()}`;
			trendMap.set(key, (trendMap.get(key) || 0) + point.count);
		}

		// Generate last 24 hours
		for (let i = HOURS - 1; i >= 0; i--) {
			const hour = new Date(now);
			hour.setMinutes(0, 0, 0);
			hour.setHours(hour.getHours() - i);
			const key = `${hour.getFullYear()}-${hour.getMonth()}-${hour.getDate()}-${hour.getHours()}`;
			result.push({
				hour,
				count: trendMap.get(key) || 0
			});
		}

		return result;
	});

	// Actual max from data
	const actualMax = $derived(() => {
		return Math.max(...hourlyData().map(d => d.count), 0);
	});

	// Scale max - at least SCALE_MAX, or higher if data exceeds it
	const scaleMax = $derived(() => {
		return Math.max(SCALE_MAX, actualMax());
	});

	// Dashed line position as percentage from bottom (actual max relative to scale max)
	const linePositionPct = $derived(() => {
		if (actualMax() === 0) return 0;
		return (actualMax() / scaleMax()) * 100;
	});
</script>

<div class="relative h-7 w-44 flex-shrink-0">
	<!-- Solid bottom line -->
	<div class="absolute left-0 right-7 bottom-0 border-t border-muted-foreground/40"></div>

	<!-- Dashed line at actual max -->
	{#if actualMax() > 0}
		<div
			class="absolute left-0 right-7 h-[1px]"
			style="bottom: {linePositionPct()}%; background: repeating-linear-gradient(to right, var(--muted-foreground) 0, var(--muted-foreground) 4px, transparent 4px, transparent 7px); opacity: 0.5;"
		></div>
		<!-- Max count label -->
		<span
			class="absolute right-1 text-[10px] w-[20px] text-muted-foreground tabular-nums"
			style="bottom: calc({linePositionPct()}% - 6px);"
		>
			{actualMax() > 999 ? (actualMax()/1000).toFixed(1) + "k" : actualMax()}
		</span>
	{/if}

	<!-- Bars container -->
	<div class="absolute inset-0 right-7 flex items-end gap-[1px]">
		{#each hourlyData() as point, i (i)}
			{@const heightPct = scaleMax() > 0 ? (point.count / scaleMax()) * 100 : 0}
			{@const hasEvents = point.count > 0}
			<Tooltip.Root>
				<Tooltip.Trigger class="flex-1 h-full flex items-end justify-center">
					<div
						class="w-full max-w-[4px] rounded-[1px] transition-colors {hasEvents ? 'hover:opacity-80' : ''}"
						style="height: {Math.max(2, heightPct)}%;{hasEvents ? ' background-color: var(--sidebar-accent);' : ''}"
					></div>
				</Tooltip.Trigger>
				<Tooltip.Content side="top" class="py-2 px-3 !animate-none !transition-none">
					<div class="font-medium text-sm">{point.count} {point.count === 1 ? 'Event' : 'Events'}</div>
					<div class="text-muted-foreground text-xs">
						{point.hour.toLocaleString(undefined, {
							month: 'short',
							day: 'numeric',
							year: 'numeric',
							hour: 'numeric',
							minute: '2-digit',
							timeZoneName: 'short'
						})}
					</div>
				</Tooltip.Content>
			</Tooltip.Root>
		{/each}
	</div>
</div>
