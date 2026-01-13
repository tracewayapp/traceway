<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Popover from '$lib/components/ui/popover';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import { Badge } from '$lib/components/ui/badge';
	import { CirclePlus, Check } from 'lucide-svelte';

	interface Props {
		availableServers: string[];
		selectedServers: string[];
		onSelectionChange?: (servers: string[]) => void;
		serverColorMap: Record<string, string>;
	}

	let {
		availableServers,
		selectedServers = $bindable([]),
		onSelectionChange,
		serverColorMap
	}: Props = $props();

	let isOpen = $state(false);
	let searchQuery = $state('');

	// Filter servers based on search
	const filteredServers = $derived(
		[...availableServers]
			.sort()
			.filter(s => s.toLowerCase().includes(searchQuery.toLowerCase()))
	);

	// Marker for "none selected" state
	const NONE_SELECTED = '__none__';

	// Check if in "none selected" state
	const isNoneSelected = $derived(
		selectedServers.length === 1 && selectedServers[0] === NONE_SELECTED
	);

	// Get the list of servers to display in the legend
	const displayServers = $derived(() => {
		if (isNoneSelected) return [];
		// If empty (all selected) or all explicitly selected, show all available servers
		if (selectedServers.length === 0 || selectedServers.length === availableServers.length) {
			return [...availableServers].sort();
		}
		// Otherwise show the selected ones
		return [...selectedServers].sort();
	});

	// Has active filters (some but not all selected, or none selected)
	const hasActiveFilters = $derived(
		isNoneSelected || (selectedServers.length > 0 && selectedServers.length < availableServers.length)
	);

	function isSelected(serverName: string): boolean {
		// None selected state
		if (isNoneSelected) return false;
		// If no servers selected, treat as "all selected"
		if (selectedServers.length === 0) return true;
		return selectedServers.includes(serverName);
	}

	function selectAll() {
		selectedServers = [];
		onSelectionChange?.([]);
	}

	function selectNone() {
		selectedServers = [NONE_SELECTED];
		onSelectionChange?.([NONE_SELECTED]);
	}

	function toggleServer(serverName: string) {
		let newSelection: string[];

		if (isNoneSelected) {
			// None selected - clicking one selects just that one
			newSelection = [serverName];
		} else if (selectedServers.length === 0) {
			// Currently "all selected" - clicking one deselects it (select all except this one)
			newSelection = availableServers.filter(s => s !== serverName);
		} else if (selectedServers.includes(serverName)) {
			// Remove from selection
			newSelection = selectedServers.filter(s => s !== serverName);
			// If removing last one, go to none selected state
			if (newSelection.length === 0) {
				newSelection = [NONE_SELECTED];
			}
		} else {
			// Add to selection
			newSelection = [...selectedServers, serverName];
			// If all are now selected, reset to empty (all selected state)
			if (newSelection.length === availableServers.length) {
				newSelection = [];
			}
		}

		selectedServers = newSelection;
		onSelectionChange?.(newSelection);
	}

	function clearFilters() {
		selectedServers = [];
		onSelectionChange?.([]);
	}

	// Reset search when popover closes
	function handleOpenChange(open: boolean) {
		if (!open) {
			searchQuery = '';
		}
		isOpen = open;
	}
</script>

<Popover.Root bind:open={isOpen} onOpenChange={handleOpenChange}>
	<Popover.Trigger>
		<Button variant="outline" size="sm" class="h-auto min-h-8 border-dashed py-1.5">
			<CirclePlus class="mr-2 h-4 w-4 flex-shrink-0" />
			<span class="flex-shrink-0">Servers</span>
			{#if displayServers().length > 0}
				<Separator orientation="vertical" class="mx-2 h-4" />
				<div class="flex flex-wrap gap-1">
					{#each displayServers() as server}
						{@const color = serverColorMap[server] || '#888888'}
						<Badge variant="secondary" class="rounded-sm px-1.5 py-0 font-normal flex items-center gap-1">
							<span
								class="h-2 w-2 rounded-full flex-shrink-0"
								style="background-color: {color};"
							></span>
							<span class="truncate max-w-[80px]">{server}</span>
						</Badge>
					{/each}
				</div>
			{:else if isNoneSelected}
				<Separator orientation="vertical" class="mx-2 h-4" />
				<span class="text-muted-foreground text-xs">None selected</span>
			{/if}
		</Button>
	</Popover.Trigger>

	<Popover.Content class="w-[200px] p-0" align="start">
		<!-- Search Input -->
		<div class="p-2">
			<Input
				placeholder="Servers"
				bind:value={searchQuery}
				class="h-8"
			/>
		</div>

		<!-- Quick Actions -->
		{#if availableServers.length > 0}
			<div class="flex items-center justify-between border-t px-2 py-1.5">
				<button
					class="text-xs text-muted-foreground hover:text-foreground"
					onclick={selectAll}
				>
					Select all
				</button>
				<button
					class="text-xs text-muted-foreground hover:text-foreground"
					onclick={selectNone}
				>
					Deselect all
				</button>
			</div>
		{/if}

		<!-- Server List -->
		<div class="max-h-[200px] overflow-y-auto">
			{#each filteredServers as serverName}
				{@const color = serverColorMap[serverName] || '#888888'}
				{@const selected = isSelected(serverName)}
				<button
					class="relative flex w-full cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground"
					onclick={() => toggleServer(serverName)}
				>
					<div class="flex h-4 w-4 items-center justify-center rounded-sm border border-primary {selected ? 'bg-primary text-primary-foreground' : 'opacity-50'}">
						{#if selected}
							<Check class="h-3 w-3" />
						{/if}
					</div>
					<span
						class="h-2 w-2 rounded-full flex-shrink-0"
						style="background-color: {color};"
					></span>
					<span class="flex-1 truncate text-left">{serverName}</span>
				</button>
			{/each}

			{#if filteredServers.length === 0}
				<div class="py-6 text-center text-sm text-muted-foreground">
					No servers found.
				</div>
			{/if}
		</div>

		<!-- Clear Filters -->
		{#if hasActiveFilters}
			<Separator />
			<button
				class="w-full py-2 text-center text-sm hover:bg-accent"
				onclick={clearFilters}
			>
				Clear filters
			</button>
		{/if}
	</Popover.Content>
</Popover.Root>
