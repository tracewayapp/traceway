<script lang="ts">
	import { cn } from "$lib/utils.js";
	import * as Table from "$lib/components/ui/table/index.js";
	import * as Tooltip from "$lib/components/ui/tooltip/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { ArrowDown, ArrowUp, ArrowUpDown, CircleQuestionMark } from "lucide-svelte";

	type SortDirection = "asc" | "desc";

	let {
		label,
		tooltip,
		sortField,
		currentSortField,
		sortDirection,
		onSort,
		align = "left",
		class: className
	}: {
		label: string;
		tooltip?: string;
		sortField?: string;
		currentSortField?: string;
		sortDirection?: SortDirection;
		onSort?: (field: string) => void;
		align?: "left" | "right";
		class?: string;
	} = $props();

	const isSortable = $derived(sortField !== undefined && onSort !== undefined);
	const isCurrentSort = $derived(isSortable && currentSortField === sortField);
	const alignRight = $derived(align === "right");
</script>

<Table.Head class={cn(alignRight && "text-right", className)}>
	{#if isSortable}
		<div class={cn("flex items-center", alignRight && "justify-end gap-1.5")}>
			<Button
				variant="ghost"
				size="sm"
				class={cn("h-8 font-medium", !alignRight && "-ml-3")}
				onclick={() => onSort?.(sortField!)}
			>
				{label}
				{#if isCurrentSort}
					{#if sortDirection === "desc"}
						<ArrowDown class="ml-2 h-4 w-4" />
					{:else}
						<ArrowUp class="ml-2 h-4 w-4" />
					{/if}
				{:else}
					<ArrowUpDown class="ml-2 h-4 w-4" />
				{/if}
			</Button>
			{#if tooltip}
				<Tooltip.Root>
					<Tooltip.Trigger>
						<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
					</Tooltip.Trigger>
					<Tooltip.Content>
						<p class="text-xs">{tooltip}</p>
					</Tooltip.Content>
				</Tooltip.Root>
			{/if}
		</div>
	{:else if tooltip}
		<span class={cn("flex items-center gap-1.5", alignRight && "justify-end")}>
			{label}
			<Tooltip.Root>
				<Tooltip.Trigger>
					<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
				</Tooltip.Trigger>
				<Tooltip.Content>
					<p class="text-xs">{tooltip}</p>
				</Tooltip.Content>
			</Tooltip.Root>
		</span>
	{:else}
		{label}
	{/if}
</Table.Head>
