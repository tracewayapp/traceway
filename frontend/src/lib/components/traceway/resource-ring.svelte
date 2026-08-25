<script lang="ts">
	interface Props {
		label: string;
		value: number | null;
		warningAt?: number;
		criticalAt?: number;
		size?: 'sm' | 'md';
	}

	let { label, value, warningAt = 80, criticalAt = 95, size = 'md' }: Props = $props();

	const normalized = $derived(value === null ? null : Math.max(0, Math.min(100, value)));
	const tone = $derived.by(() => {
		if (normalized === null) return 'text-muted-foreground/35';
		if (normalized >= criticalAt) return 'text-crit';
		if (normalized >= warningAt) return 'text-amber-500 dark:text-amber-400';
		return 'text-ok';
	});
	const dimension = $derived(size === 'sm' ? 'size-11' : 'size-13');
	const valueText = $derived(normalized === null ? '—' : `${Math.round(normalized)}`);
	const accessibleValue = $derived(
		normalized === null ? `${label}: no data` : `${label}: ${normalized.toFixed(1)} percent`
	);
</script>

<div
	class="relative grid {dimension} shrink-0 place-items-center"
	role="img"
	aria-label={accessibleValue}
	title={accessibleValue}
>
	<svg class="absolute inset-0 size-full -rotate-90" viewBox="0 0 48 48" aria-hidden="true">
		<circle
			cx="24"
			cy="24"
			r="19"
			pathLength="100"
			fill="none"
			stroke="currentColor"
			stroke-width="4"
			class="text-muted"
		/>
		{#if normalized !== null}
			<circle
				cx="24"
				cy="24"
				r="19"
				pathLength="100"
				fill="none"
				stroke="currentColor"
				stroke-width="4"
				stroke-linecap="round"
				stroke-dasharray="{normalized} {100 - normalized}"
				class={tone}
			/>
		{/if}
	</svg>
	<span class="font-mono text-[11px] font-semibold tracking-tight tabular-nums">
		{valueText}{#if normalized !== null}<span class="text-[8px] text-muted-foreground">%</span>{/if}
	</span>
</div>
