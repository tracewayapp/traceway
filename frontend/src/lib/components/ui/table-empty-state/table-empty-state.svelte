<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';
	import * as Table from '$lib/components/ui/table/index.js';

	let {
		colspan,
		message = 'No data found.',
		class: className,
		children
	}: {
		colspan: number;
		message?: string;
		class?: string;
		children?: Snippet;
	} = $props();
</script>

<Table.Row>
	<Table.Cell
		{colspan}
		class={cn(
			'h-45 px-6 text-center break-words whitespace-normal text-muted-foreground',
			className
		)}
	>
		{@render children?.()}
		{message}
	</Table.Cell>
</Table.Row>
