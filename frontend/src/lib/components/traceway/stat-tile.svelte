<script lang="ts">
	import { cn } from '$lib/utils.js';

	interface Props {
		label: string;
		value: string | number;
		sub?: string;
		valueClass?: string;
		class?: string;
	}

	let { label, value, sub, valueClass, class: className }: Props = $props();
</script>

<div class={cn('px-6 py-4', className)}>
	<div class="text-xs font-medium text-muted-foreground">{label}</div>
	<div class={cn('mt-1 text-2xl font-semibold tracking-tight tabular-nums', valueClass)}>
		{value}
	</div>
	{#if sub}
		<div class="mt-0.5 text-xs text-muted-foreground">{sub}</div>
	{/if}
</div>
