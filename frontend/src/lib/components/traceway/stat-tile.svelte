<script lang="ts">
	import { cn } from '$lib/utils.js';

	interface Props {
		label: string;
		value: string | number;
		sub?: string;
		valueClass?: string;
	}

	let { label, value, sub, valueClass }: Props = $props();
</script>

<div class="px-6 py-4">
	<div class="text-xs font-medium text-muted-foreground">{label}</div>
	<div class={cn('mt-1 text-2xl font-semibold tracking-tight tabular-nums', valueClass)}>
		{value}
	</div>
	{#if sub}
		<div class="mt-0.5 text-xs text-muted-foreground">{sub}</div>
	{/if}
</div>
