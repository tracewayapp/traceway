<script lang="ts">
	import { cn } from '$lib/utils.js';

	let {
		label,
		value,
		fallback = '-',
		mono = false,
		truncate = false,
		large = false,
		valueClass
	}: {
		label: string;
		value: string | number | null | undefined;
		fallback?: string;
		mono?: boolean;
		truncate?: boolean;
		large?: boolean;
		valueClass?: string;
	} = $props();

	const displayValue = $derived(value ?? fallback);
</script>

<div class="space-y-1">
	<p class="text-sm text-muted-foreground">{label}</p>
	<p
		class={cn(
			!large && 'text-sm',
			mono && 'font-mono',
			truncate ? 'truncate' : 'break-all',
			valueClass
		)}
		title={truncate && typeof displayValue === 'string' ? displayValue : undefined}
	>
		{displayValue}
	</p>
</div>
