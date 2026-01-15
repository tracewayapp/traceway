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
	import { getTimezone } from '$lib/state/timezone.svelte';
	import IssueTrendChart from '$lib/components/issue-trend-chart.svelte';
	import Archive from '@lucide/svelte/icons/archive';
	import { toast } from 'svelte-sonner';
	import ArchiveConfirmationDialog from '$lib/components/archive-confirmation-dialog.svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { formatDateTime, getNow, toUTCISO } from '$lib/utils/formatters';

	const timezone = $derived(getTimezone());

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
	let showArchiveDialog = $state(false);

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
			const now = getNow(timezone);
			const fromDate = now.minus({ days: parseInt(daysBack) });

			const requestBody = {
				fromDate: toUTCISO(fromDate),
				toDate: toUTCISO(now),
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

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
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

			toast.success('Successfully archived the Issue' + (selectedHashes.size > 1 ? 's' : ''));
			selectedHashes = new Set();
			await loadData();
		} catch (e: any) {
			console.error('Archive failed:', e);
			error = e.message || 'Failed to archive issues';
			throw e;
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
				onclick={() => showArchiveDialog = true}
				disabled={archiving}
				class="gap-1.5"
			>
				<Archive class="h-4 w-4" />
				Archive
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
					<TableEmptyState colspan={5} message="No issues found." />
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
						<TracewayTableHeader
							label="Issue"
							tooltip="The error message or exception that occurred"
						/>
						<TracewayTableHeader
							label="Trend"
							tooltip="Hourly occurrence pattern over the last 24h"
							class="w-[190px]"
						/>
						<TracewayTableHeader
							label="Events"
							tooltip="Total number of times this issue occurred in the selected range"
							align="right"
							class="w-[80px]"
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When this issue last occurred"
							class="w-[180px]"
						/>
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
								{formatDateTime(exception.lastSeen, { timezone })}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<!-- Pagination Footer -->
	<PaginationFooter
		currentPage={page}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		{loading}
		itemLabel="issue"
	/>
</div>

<ArchiveConfirmationDialog
	open={showArchiveDialog}
	onOpenChange={(open) => showArchiveDialog = open}
	count={selectedCount}
	onConfirm={archiveSelected}
/>
