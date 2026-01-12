<script lang="ts">
	import { onMount } from 'svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { api } from '$lib/api';
	import * as Table from '$lib/components/ui/table';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { projectsState } from '$lib/state/projects.svelte';
	import IssueTrendChart from '$lib/components/issue-trend-chart.svelte';
	import Archive from '@lucide/svelte/icons/archive';
	import { CircleQuestionMark } from '@lucide/svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';

	type ExceptionTrendPoint = {
		timestamp: string;
		count: number;
	};

	type ExceptionGroup = {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
		hourlyTrend: ExceptionTrendPoint[];
	};

	let exceptions = $state<ExceptionGroup[]>([]);
	let loading = $state(true);
	let error = $state('');
	let archiving = $state(false);

	// Pagination State
	let page = $state(1);
	let pageSize = $state(10);
	let total = $state(0);
	let totalPages = $state(0);

	// Selection State
	let selectedHashes = $state<Set<string>>(new Set());

	// Derived selection states
	const selectedCount = $derived(selectedHashes.size);
	const allSelected = $derived(exceptions.length > 0 && selectedHashes.size === exceptions.length);
	const someSelected = $derived(selectedHashes.size > 0 && selectedHashes.size < exceptions.length);

	// Filters
	let searchQuery = $state('');
	let daysBack = $state('7');

	// Select options for days back
	const daysOptions = [
		{ value: '1', label: '24 Hours' },
		{ value: '7', label: '7 Days' }
	];

	// Page size options
	const pageSizeOptions = [
		{ value: '10', label: '10' },
		{ value: '20', label: '20' },
		{ value: '50', label: '50' },
		{ value: '100', label: '100' }
	];

	// Derived labels for select displays
	const daysBackLabel = $derived(
		daysOptions.find((o) => o.value === daysBack)?.label ?? 'Select period'
	);
	const pageSizeLabel = $derived(
		pageSizeOptions.find((o) => o.value === pageSize.toString())?.label ?? pageSize.toString()
	);

	async function loadData() {
		loading = true;
		error = '';

		try {
			const now = new Date();
			const fromDate = new Date();
			fromDate.setDate(now.getDate() - parseInt(daysBack));

			const requestBody = {
				fromDate: fromDate.toISOString(),
				toDate: now.toISOString(),
				orderBy: 'last_seen',
				pagination: {
					page: page,
					pageSize: pageSize
				},
				search: searchQuery.trim(),
				includeArchived: false
			};

			const response = await api.post('/exception-stack-traces', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			exceptions = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;

			// Clear selection when data changes
			selectedHashes = new Set();
		} catch (e: any) {
			console.error(e);
			error = e.message || 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadData();
		}
	}

	function handlePageSizeChange(newPageSize: string) {
		pageSize = parseInt(newPageSize);
		page = 1;
		loadData();
	}

	// Debounce search input
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			page = 1;
			loadData();
		}, 300);
	}

	// Selection handlers
	function toggleSelectAll() {
		if (allSelected) {
			selectedHashes = new Set();
		} else {
			selectedHashes = new Set(exceptions.map((e) => e.exceptionHash));
		}
	}

	function toggleSelect(hash: string) {
		const newSet = new Set(selectedHashes);
		if (newSet.has(hash)) {
			newSet.delete(hash);
		} else {
			newSet.add(hash);
		}
		selectedHashes = newSet;
	}

	function isSelected(hash: string): boolean {
		return selectedHashes.has(hash);
	}

	// Archive handler
	async function archiveSelected() {
		if (selectedHashes.size === 0) return;

		archiving = true;
		try {
			await api.post(
				'/exception-stack-traces/archive',
				{
					hashes: Array.from(selectedHashes)
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);

			selectedHashes = new Set();
			await loadData();
		} catch (e: any) {
			console.error('Archive failed:', e);
			error = e.message || 'Failed to archive issues';
		} finally {
			archiving = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-4">
	<!-- Header with Title and Search + Period Filter -->
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="text-2xl font-bold tracking-tight">Issues</h2>
		<div class="flex">
			<Input
				placeholder="Search exceptions..."
				class="h-9 w-[250px] rounded-r-none border-r-0 focus:relative focus:z-10 lg:w-[320px]"
				bind:value={searchQuery}
				oninput={handleSearchInput}
			/>
			<Select.Root
				type="single"
				bind:value={daysBack}
				onValueChange={(v) => {
					if (v) {
						page = 1;
						loadData();
					}
				}}
			>
				<Select.Trigger class="-ml-px h-9 w-[110px] rounded-l-none focus:relative focus:z-10">
					{daysBackLabel}
				</Select.Trigger>
				<Select.Content>
					{#each daysOptions as option}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
	</div>

	<!-- Archive Toolbar - shown when items selected -->
	{#if selectedCount > 0}
		<div
			class="flex animate-in items-center gap-3 rounded-lg border bg-muted/50 p-3 duration-200 fade-in slide-in-from-top-1"
		>
			<span class="text-sm font-medium"
				>{selectedCount} issue{selectedCount === 1 ? '' : 's'} selected</span
			>
			<Button
				variant="outline"
				size="sm"
				onclick={archiveSelected}
				disabled={archiving}
				class="gap-1.5"
			>
				<Archive class="h-4 w-4" />
				{archiving ? 'Archiving...' : 'Archive'}
			</Button>
			<Button variant="ghost" size="sm" onclick={() => (selectedHashes = new Set())}>
				Clear selection
			</Button>
		</div>
	{/if}

	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={5} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={5} class="h-24 text-center text-red-500">
							{error}
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if exceptions.length === 0}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={5} class="h-24 text-center text-muted-foreground">
							No issues found.
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-[40px] pl-4">
							<Checkbox
								checked={allSelected ? true : someSelected ? 'indeterminate' : false}
								onCheckedChange={toggleSelectAll}
								aria-label="Select all"
							/>
						</Table.Head>
						<Table.Head>
							<span class="flex items-center gap-1.5">
								Issue
								<Tooltip.Root>
									<Tooltip.Trigger>
										<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="text-xs">The error message or exception that occurred</p>
									</Tooltip.Content>
								</Tooltip.Root>
							</span>
						</Table.Head>
						<Table.Head class="w-[190px]">
							<span class="flex items-center gap-1.5">
								Trend
								<Tooltip.Root>
									<Tooltip.Trigger>
										<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="text-xs">Hourly occurrence pattern over the last 24h</p>
									</Tooltip.Content>
								</Tooltip.Root>
							</span>
						</Table.Head>
						<Table.Head class="w-[80px] text-right">
							<span class="flex items-center justify-end gap-1.5">
								Events
								<Tooltip.Root>
									<Tooltip.Trigger>
										<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="text-xs">
											Total number of times this issue occurred in the selected range
										</p>
									</Tooltip.Content>
								</Tooltip.Root>
							</span>
						</Table.Head>
						<Table.Head class="w-[180px]">
							<span class="flex items-center gap-1.5">
								Last Seen
								<Tooltip.Root>
									<Tooltip.Trigger>
										<CircleQuestionMark class="h-3.5 w-3.5 text-muted-foreground/60" />
									</Tooltip.Trigger>
									<Tooltip.Content>
										<p class="text-xs">When this issue last occurred</p>
									</Tooltip.Content>
								</Tooltip.Root>
							</span>
						</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each exceptions as exception (exception.exceptionHash)}
						<Table.Row
							class="group cursor-pointer hover:bg-muted/50"
							data-state={isSelected(exception.exceptionHash) ? 'selected' : undefined}
						>
							<Table.Cell class="pl-4" onclick={(e) => e.stopPropagation()}>
								<Checkbox
									checked={isSelected(exception.exceptionHash)}
									onCheckedChange={() => toggleSelect(exception.exceptionHash)}
									aria-label="Select row"
								/>
							</Table.Cell>
							<Table.Cell
								class="max-w-[400px] truncate font-mono text-sm"
								title={exception.stackTrace}
								onclick={createRowClickHandler(`/issues/${exception.exceptionHash}`)}
							>
								<span class="text-foreground">{exception.stackTrace.split('\n')[0]}</span>
							</Table.Cell>
							<Table.Cell onclick={createRowClickHandler(`/issues/${exception.exceptionHash}`)}>
								<IssueTrendChart trend={exception.hourlyTrend || []} />
							</Table.Cell>
							<Table.Cell
								class="text-right font-medium tabular-nums"
								onclick={createRowClickHandler(`/issues/${exception.exceptionHash}`)}
							>
								{exception.count.toLocaleString()}
							</Table.Cell>
							<Table.Cell
								class="text-muted-foreground"
								onclick={createRowClickHandler(`/issues/${exception.exceptionHash}`)}
							>
								{new Date(exception.lastSeen).toLocaleString()}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<!-- Pagination Footer -->
	<div class="flex items-center justify-between px-2">
		<div class="flex-1 text-sm text-muted-foreground">
			{#if selectedCount > 0}
				{selectedCount} of {total} row(s) selected.
			{:else}
				{total} issue{total === 1 ? '' : 's'} total
			{/if}
		</div>
		<div class="flex items-center space-x-6 lg:space-x-8">
			<div class="flex items-center space-x-2">
				<p class="text-sm font-medium">Rows per page</p>
				<Select.Root
					type="single"
					value={pageSize.toString()}
					onValueChange={(v) => {
						if (v) {
							handlePageSizeChange(v);
						}
					}}
				>
					<Select.Trigger class="h-8 w-[70px]">
						{pageSizeLabel}
					</Select.Trigger>
					<Select.Content side="top">
						{#each pageSizeOptions as option}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			<div class="flex w-[100px] items-center justify-center text-sm font-medium">
				Page {page} of {totalPages || 1}
			</div>
			<div class="flex items-center space-x-2">
				<Button
					variant="outline"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={() => handlePageChange(page - 1)}
					disabled={page <= 1 || loading}
				>
					<span class="sr-only">Go to previous page</span>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="15"
						height="15"
						viewBox="0 0 15 15"
						fill="none"
						class="lucide lucide-chevron-left h-4 w-4"
					>
						<path
							d="M8.84182 3.13514C9.04327 3.32401 9.05348 3.64042 8.86462 3.84188L5.43521 7.49991L8.86462 11.1579C9.05348 11.3594 9.04327 11.6758 8.84182 11.8647C8.64036 12.0535 8.32394 12.0433 8.13508 11.8419L4.38508 7.84188C4.20477 7.64955 4.20477 7.35027 4.38508 7.15794L8.13508 3.15794C8.32394 2.95648 8.64036 2.94628 8.84182 3.13514Z"
							fill="currentColor"
							fill-rule="evenodd"
							clip-rule="evenodd"
						></path>
					</svg>
				</Button>
				<Button
					variant="outline"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={() => handlePageChange(page + 1)}
					disabled={page >= totalPages || loading}
				>
					<span class="sr-only">Go to next page</span>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="15"
						height="15"
						viewBox="0 0 15 15"
						fill="none"
						class="lucide lucide-chevron-right h-4 w-4"
					>
						<path
							d="M6.1584 3.13508C6.35985 2.94621 6.67627 2.95642 6.86514 3.15788L10.6151 7.15788C10.7954 7.3502 10.7954 7.64949 10.6151 7.84182L6.86514 11.8418C6.67627 12.0433 6.35985 12.0535 6.1584 11.8646C5.95694 11.6757 5.94673 11.3593 6.1356 11.1579L9.565 7.49985L6.1356 3.84182C5.94673 3.64036 5.95694 3.32394 6.1584 3.13508Z"
							fill="currentColor"
							fill-rule="evenodd"
							clip-rule="evenodd"
						></path>
					</svg>
				</Button>
			</div>
		</div>
	</div>
</div>
