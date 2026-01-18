<script lang="ts">
	import { cn } from "$lib/utils.js";
	import * as Table from "$lib/components/ui/table/index.js";

	let {
		colspan,
		message = "No data found.",
		class: className
	}: {
		colspan: number;
		message?: string;
		class?: string;
	} = $props();
</script>

<Table.Row>
	<Table.Cell {colspan} class={cn("h-45 text-center text-muted-foreground", className)}>
		{message}
	</Table.Cell>
</Table.Row>
