<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { ArrowRight } from '@lucide/svelte';
	import { createRowClickHandlerWithNavigate } from '$lib/utils/navigation';

	let {
		colspan,
		href,
		label,
		onBeforeNavigate
	}: {
		colspan: number;
		href: string;
		label: string;
		onBeforeNavigate?: () => void;
	} = $props();
</script>

<Table.Row
	class="cursor-pointer bg-muted/50 hover:bg-muted"
	onclick={createRowClickHandlerWithNavigate(href, onBeforeNavigate)}
>
	<Table.Cell {colspan} class="py-2 text-center text-sm text-muted-foreground">
		{label}
		<ArrowRight class="inline h-3.5 w-3.5" />
	</Table.Cell>
</Table.Row>
