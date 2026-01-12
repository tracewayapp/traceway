<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Popover from '$lib/components/ui/popover';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Server, ChevronDown } from 'lucide-svelte';
	import { getServerColor } from '$lib/utils/server-colors';

	interface Props {
		availableServers: string[];
		selectedServers: string[];
		onSelectionChange?: (servers: string[]) => void;
	}

	let {
		availableServers,
		selectedServers = $bindable([]),
		onSelectionChange
	}: Props = $props();

	let isOpen = $state(false);
	let tempSelectedServers = $state<Set<string>>(new Set(selectedServers));

	// Sync temp state when popover opens
	$effect(() => {
		if (isOpen) {
			tempSelectedServers = new Set(selectedServers);
		}
	});

	// Computed states
	const allSelected = $derived(tempSelectedServers.size === availableServers.length);
	const noneSelected = $derived(tempSelectedServers.size === 0);

	// Display label for the trigger button
	const displayLabel = $derived(() => {
		if (selectedServers.length === 0 || selectedServers.length === availableServers.length) {
			return 'All servers';
		}
		if (selectedServers.length === 1) {
			return selectedServers[0];
		}
		return `${selectedServers.length} servers`;
	});

	function toggleServer(serverName: string) {
		const newSet = new Set(tempSelectedServers);
		if (newSet.has(serverName)) {
			newSet.delete(serverName);
		} else {
			newSet.add(serverName);
		}
		tempSelectedServers = newSet;
	}

	function selectAll() {
		tempSelectedServers = new Set(availableServers);
	}

	function clearAll() {
		tempSelectedServers = new Set();
	}

	function handleOpenChange(open: boolean) {
		if (!open) {
			// Apply selection when closing
			const newSelection = Array.from(tempSelectedServers);
			selectedServers = newSelection;
			onSelectionChange?.(newSelection);
		}
		isOpen = open;
	}

	function isSelected(serverName: string): boolean {
		return tempSelectedServers.has(serverName);
	}
</script>

<Popover.Root bind:open={isOpen} onOpenChange={handleOpenChange}>
	<Popover.Trigger>
		<Button variant="outline" class="h-9 justify-between gap-2 font-normal">
			<span class="flex items-center gap-2">
				<Server class="h-4 w-4 text-muted-foreground" />
				<span class="truncate">{displayLabel()}</span>
			</span>
			<ChevronDown class="h-4 w-4 text-muted-foreground" />
		</Button>
	</Popover.Trigger>
	<Popover.Content class="w-[220px] p-0" align="start">
		<div class="p-3 border-b">
			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Servers</span>
				<div class="flex items-center gap-2">
					<button
						class="text-xs text-primary hover:underline disabled:text-muted-foreground disabled:no-underline"
						onclick={selectAll}
						disabled={allSelected}
					>
						All
					</button>
					<span class="text-muted-foreground">|</span>
					<button
						class="text-xs text-primary hover:underline disabled:text-muted-foreground disabled:no-underline"
						onclick={clearAll}
						disabled={noneSelected}
					>
						Clear
					</button>
				</div>
			</div>
		</div>
		<div class="max-h-[280px] overflow-y-auto py-2">
			{#each availableServers.sort() as serverName}
				{@const color = getServerColor(serverName, availableServers)}
				<button
					class="w-full px-3 py-1.5 text-left text-sm hover:bg-muted/50 flex items-center gap-3 transition-colors"
					onclick={() => toggleServer(serverName)}
				>
					<Checkbox
						checked={isSelected(serverName)}
						aria-label={`Select ${serverName}`}
					/>
					<span
						class="w-3 h-3 rounded-full flex-shrink-0"
						style="background-color: {color};"
					></span>
					<span class="truncate">{serverName}</span>
				</button>
			{/each}
		</div>
		<div class="p-3 border-t">
			<Button size="sm" class="w-full" onclick={() => isOpen = false}>
				Apply
			</Button>
		</div>
	</Popover.Content>
</Popover.Root>
