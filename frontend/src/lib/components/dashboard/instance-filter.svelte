<script lang="ts">
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import * as Command from '$lib/components/ui/command';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Popover from '$lib/components/ui/popover';
	import { Check, ChevronDown, Server, X } from '@lucide/svelte';

	let {
		value = '',
		from,
		to,
		projectId,
		onValueChange
	} = $props<{
		value?: string;
		from: string;
		to: string;
		projectId?: string | null;
		onValueChange?: (value: string) => void;
	}>();

	let open = $state(false);
	let search = $state('');
	let instances = $state<string[]>([]);
	let loading = $state(false);
	let loadError = $state('');
	let loadedKey = '';
	let pendingKey = '';
	let observedContext = '';
	let loadSequence = 0;

	const selectedIsOutsideRange = $derived(value !== '' && !instances.includes(value));

	function contextKey() {
		return `${projectId ?? ''}|${from}|${to}`;
	}

	async function loadInstances(force = false) {
		if (!projectId) {
			instances = [];
			loading = false;
			loadError = '';
			return;
		}

		const key = contextKey();
		if (!force && (loadedKey === key || pendingKey === key)) return;

		const sequence = ++loadSequence;
		pendingKey = key;
		loading = true;
		loadError = '';

		try {
			const response = await api.get(
				`/metrics/discover/instances?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
				{ projectId }
			);
			if (sequence !== loadSequence) return;

			const discovered = Array.isArray(response.instances) ? response.instances : [];
			instances = [...new Set(discovered)]
				.filter(
					(instance): instance is string => typeof instance === 'string' && instance.trim() !== ''
				)
				.map((instance) => instance.trim())
				.sort((a, b) => a.localeCompare(b));
			loadedKey = key;
		} catch {
			if (sequence !== loadSequence) return;
			instances = [];
			loadError = 'Could not load instances for this time range.';
		} finally {
			if (sequence === loadSequence) {
				pendingKey = '';
				loading = false;
			}
		}
	}

	function handleOpenChange(nextOpen: boolean) {
		open = nextOpen;
		if (nextOpen) {
			void loadInstances();
		} else {
			search = '';
		}
	}

	function selectInstance(instance: string) {
		onValueChange?.(instance);
		open = false;
		search = '';
	}

	$effect(() => {
		const key = contextKey();
		if (key === observedContext) return;

		observedContext = key;
		loadedKey = '';
		instances = [];
		loadError = '';
		loadSequence++;
		pendingKey = '';
		loading = false;
		if (open) void loadInstances();
	});
</script>

<div class="flex h-9 w-full min-w-0 @2xl:w-60 @2xl:min-w-60 @2xl:shrink-0">
	<Popover.Root bind:open onOpenChange={handleOpenChange}>
		<Popover.Trigger class="min-w-0 flex-1">
			<Button
				variant="outline"
				class="h-9 w-full min-w-0 shrink justify-start gap-2 px-3 font-normal {value
					? 'rounded-r-none border-primary/30 bg-primary/5 hover:bg-primary/10 dark:bg-primary/10'
					: 'border-dashed text-muted-foreground'}"
				aria-label={value ? `Instance filter: ${value}` : 'Filter dashboard by instance'}
			>
				<Server class="size-4 shrink-0 {value ? 'text-primary' : ''}" />
				{#if value}
					<span
						class="min-w-0 flex-1 truncate text-left font-mono text-xs font-medium text-foreground"
					>
						{value}
					</span>
				{:else}
					<span class="min-w-0 flex-1 truncate text-left">All instances</span>
				{/if}
				<ChevronDown class="size-3.5 shrink-0 text-muted-foreground" />
			</Button>
		</Popover.Trigger>

		<Popover.Content align="end" class="w-80 max-w-[calc(100vw-2rem)] p-0">
			<div class="border-b px-3 py-2.5">
				<p class="text-sm font-medium">Filter by instance</p>
				<p class="mt-0.5 text-xs text-muted-foreground">
					Scopes every metric query on this dashboard.
				</p>
			</div>

			{#if loading}
				<div class="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
					<LoadingCircle size="sm" />
					<span>Loading instances...</span>
				</div>
			{:else if loadError}
				<div class="space-y-2 px-4 py-5 text-center">
					<p class="text-sm text-muted-foreground">{loadError}</p>
					<Button variant="outline" size="sm" onclick={() => loadInstances(true)}>Try again</Button>
				</div>
			{:else}
				<Command.Root>
					<Command.Input bind:value={search} placeholder="Find an instance..." />
					<Command.List class="max-h-72">
						<Command.Empty>No matching instances.</Command.Empty>
						<Command.Group>
							<Command.Item
								value="all instances"
								keywords={['all', 'clear', 'project']}
								onSelect={() => selectInstance('')}
							>
								<div class="grid size-6 shrink-0 place-items-center rounded-md bg-muted">
									<Server class="size-3.5 text-muted-foreground" />
								</div>
								<span class="flex-1">All instances</span>
								{#if value === ''}<Check class="text-primary" />{/if}
							</Command.Item>
						</Command.Group>

						{#if selectedIsOutsideRange}
							<Command.Separator />
							<Command.Group heading="Selected">
								<Command.Item {value} onSelect={() => selectInstance(value)}>
									<span class="size-2 shrink-0 rounded-full bg-muted-foreground/50"></span>
									<span class="min-w-0 flex-1 truncate font-mono text-xs">{value}</span>
									<span class="text-[11px] text-muted-foreground">Not seen</span>
									<Check class="text-primary" />
								</Command.Item>
							</Command.Group>
						{/if}

						{#if instances.length > 0}
							<Command.Separator />
							<Command.Group heading={`Seen in range · ${instances.length}`}>
								{#each instances as instance (instance)}
									<Command.Item value={instance} onSelect={() => selectInstance(instance)}>
										<span class="size-2 shrink-0 rounded-full bg-emerald-500"></span>
										<span class="min-w-0 flex-1 truncate font-mono text-xs">{instance}</span>
										{#if value === instance}<Check class="text-primary" />{/if}
									</Command.Item>
								{/each}
							</Command.Group>
						{:else if search === '' && !selectedIsOutsideRange}
							<div class="px-4 py-6 text-center text-sm text-muted-foreground">
								No instances reported in this time range.
							</div>
						{/if}
					</Command.List>
				</Command.Root>
			{/if}
		</Popover.Content>
	</Popover.Root>

	{#if value}
		<Button
			variant="outline"
			size="icon"
			class="-ml-px size-9 rounded-l-none border-primary/30 bg-primary/5 text-muted-foreground hover:bg-primary/10 hover:text-foreground dark:bg-primary/10"
			onclick={() => selectInstance('')}
			aria-label={`Clear instance filter for ${value}`}
			title="Clear instance filter"
		>
			<X class="size-3.5" />
		</Button>
	{/if}
</div>
