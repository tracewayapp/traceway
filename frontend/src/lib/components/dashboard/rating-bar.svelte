<script lang="ts">
	import type { RatingShare } from '$lib/utils/metric-widgets';

	let { shares } = $props<{ shares: RatingShare[] }>();

	// The app's status colours. A rating is a state, so it never borrows a series colour.
	const colors: Record<RatingShare['key'], string> = {
		good: 'var(--ok)',
		'needs-improvement': 'var(--amber)',
		poor: 'var(--crit)'
	};

	const total = $derived(shares.reduce((sum: number, s: RatingShare) => sum + s.count, 0));
	const visible = $derived(shares.filter((s: RatingShare) => s.share > 0));

	function percent(share: number): string {
		const value = share * 100;
		return `${value >= 10 || value === 0 ? Math.round(value) : value.toFixed(1)}%`;
	}

	function count(n: number): string {
		return Math.round(n).toLocaleString();
	}
</script>

{#if total > 0}
	<div class="flex h-full flex-col justify-center gap-4 px-2">
		<div
			class="flex h-6 w-full gap-0.5 overflow-hidden rounded"
			role="img"
			aria-label={shares.map((s: RatingShare) => `${s.label} ${percent(s.share)}`).join(', ')}
		>
			{#each visible as share (share.key)}
				<div
					class="h-full min-w-1 first:rounded-l last:rounded-r"
					style="width: {share.share * 100}%; background-color: {colors[
						share.key as RatingShare['key']
					]};"
					title="{share.label}: {percent(share.share)} ({count(share.count)})"
				></div>
			{/each}
		</div>
		<dl class="grid grid-cols-3 gap-2">
			{#each shares as share (share.key)}
				<div class="min-w-0">
					<dt class="flex items-center gap-1.5 text-xs text-muted-foreground">
						<span
							class="h-2 w-2 shrink-0 rounded-full"
							style="background-color: {colors[share.key as RatingShare['key']]};"
						></span>
						<span class="truncate">{share.label}</span>
					</dt>
					<dd class="mt-0.5 text-lg font-semibold tabular-nums">{percent(share.share)}</dd>
					<dd class="text-xs text-muted-foreground tabular-nums">{count(share.count)}</dd>
				</div>
			{/each}
		</dl>
	</div>
{:else}
	<div class="flex h-full items-center justify-center text-sm text-muted-foreground">No data</div>
{/if}
