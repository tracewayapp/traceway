<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import SloKindBadge from '$lib/components/slos/slo-kind-badge.svelte';
	import SloStatusBadge from '$lib/components/slos/slo-status-badge.svelte';
	import SloSheet from './slo-sheet.svelte';
	import { browser } from '$app/environment';
	import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import {
		slosState,
		SLI_KIND_LABELS,
		BACKEND_SLI_KINDS,
		FRONTEND_SLI_KINDS,
		type SLO
	} from '$lib/state/slos.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import { updateUrl } from '$lib/utils/url-params';
	import {
		budgetTone,
		burnTone,
		formatBudgetRemaining,
		formatBurnRate,
		formatSloPercent,
		sliSummary,
		toneBarClass,
		toneTextClass
	} from '$lib/utils/slo-math';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';
	import { Plus } from '@lucide/svelte';

	type SortField =
		| 'name'
		| 'kind'
		| 'target'
		| 'window'
		| 'current_sli'
		| 'budget'
		| 'burn_1h'
		| 'status';

	let slos = $state<SLO[]>([]);
	let loading = $state(true);
	let error = $state('');
	let sheetOpen = $state(false);

	const isFrontendProject = $derived(
		!!projectsState.currentProject && isFrontendFramework(projectsState.currentProject.framework)
	);
	const kindOptions = $derived([
		{ value: '', label: 'All kinds' },
		...(isFrontendProject ? FRONTEND_SLI_KINDS : BACKEND_SLI_KINDS).map((kind) => ({
			value: kind,
			label: SLI_KIND_LABELS[kind]
		}))
	]);
	let kindFilter = $state('');

	function parseSearchFromUrl(): string {
		if (!browser) return '';
		return new URLSearchParams(window.location.search).get('search') || '';
	}

	let searchQuery = $state(parseSearchFromUrl());

	function updateSearchUrl(pushToHistory = true) {
		updateUrl({ search: searchQuery.trim() || null }, { pushToHistory });
	}

	function handlePopState() {
		searchQuery = parseSearchFromUrl();
		loadData(false);
	}

	const SORT_STORAGE_KEY = 'slos';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'status', direction: 'asc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';
		updateSearchUrl(pushToHistory);
		try {
			const response = await api.get('/slos', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			slos = response.slos || [];
			slosState.refreshBreachedCount();
		} catch (e) {
			console.error(e);
			error = getErrorMessage(e) || 'Failed to load SLOs';
		} finally {
			loading = false;
		}
	}

	function handleSort(field: SortField) {
		const newSort = handleSortClick(field, orderBy, sortDirection);
		orderBy = newSort.field as SortField;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
	}

	function handleSearch() {
		updateSearchUrl(true);
	}

	const statusRank: Record<string, number> = { breached: 0, no_data: 1, ok: 2 };

	function numeric(value: number | null): number {
		return value === null ? Number.NEGATIVE_INFINITY : value;
	}

	const visibleSlos = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		let filtered = kindFilter ? slos.filter((s) => s.sliKind === kindFilter) : slos;
		if (query) {
			filtered = filtered.filter(
				(s) =>
					s.name.toLowerCase().includes(query) ||
					s.description.toLowerCase().includes(query) ||
					sliSummary(s).toLowerCase().includes(query)
			);
		}
		const direction = sortDirection === 'asc' ? 1 : -1;
		return [...filtered].sort((a, b) => {
			switch (orderBy) {
				case 'kind':
					return direction * a.sliKind.localeCompare(b.sliKind);
				case 'target':
					return direction * (a.targetPercent - b.targetPercent);
				case 'window':
					return direction * (a.windowDays - b.windowDays);
				case 'current_sli':
					return direction * (numeric(a.currentSli) - numeric(b.currentSli));
				case 'budget':
					return direction * (numeric(a.budgetRemaining) - numeric(b.budgetRemaining));
				case 'burn_1h':
					return direction * (numeric(a.burnRate1h) - numeric(b.burnRate1h));
				case 'status': {
					const rankA = a.enabled ? statusRank[a.status] : 3;
					const rankB = b.enabled ? statusRank[b.status] : 3;
					return direction * (rankA - rankB) || a.name.localeCompare(b.name);
				}
				default:
					return direction * a.name.localeCompare(b.name);
			}
		});
	});

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadData(false);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<PageHeader title="SLOs">
		{#snippet actions()}
			{#if projectsState.canWriteCurrentProject}
				<Button variant="success" size="sm" onclick={() => (sheetOpen = true)}>
					<Plus class="mr-2 h-4 w-4" />
					New SLO
				</Button>
			{/if}
		{/snippet}
	</PageHeader>

	<InfoCallout>
		A service level objective holds an indicator, the share of good requests, spans, task runs,
		probes, metric slices or crash-free sessions, to a target over a rolling window. The error
		budget is what the target leaves over, and the burn rate says how fast it is being spent.
	</InfoCallout>

	<SearchBar
		placeholder="Search SLOs..."
		bind:value={searchQuery}
		bind:typeValue={kindFilter}
		typeOptions={kindOptions}
		onSearch={handleSearch}
		disabled={loading}
	/>

	{#if loading}
		<div class="flex h-48 items-center justify-center">
			<LoadingCircle size="xlg" />
		</div>
	{:else if error}
		<ErrorDisplay status={400} title="Error" description={error} onRetry={() => loadData(false)} />
	{:else if slos.length === 0}
		{#if projectsState.canWriteCurrentProject}
			<EmptyState
				message={isFrontendProject
					? 'No SLOs yet. Define a crash-free sessions target to start tracking its error budget.'
					: 'No SLOs yet. Define a target for an endpoint, span, task, monitor or metric to start tracking its error budget.'}
				actionLabel="Create your first SLO"
				onAction={() => (sheetOpen = true)}
			/>
		{:else}
			<EmptyState message="No SLOs yet." />
		{/if}
	{:else}
		<TableContainer minWidth="960px" empty={visibleSlos.length === 0}>
			<Table.Root>
				{#if visibleSlos.length === 0}
					<Table.Body>
						<TableEmptyState colspan={8} message="No SLOs match your search." />
					</Table.Body>
				{:else}
					<Table.Header>
						<Table.Row>
							<TracewayTableHeader
								label="Name"
								sortField="name"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
							/>
							<TracewayTableHeader
								label="SLI"
								sortField="kind"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[110px]"
							/>
							<TracewayTableHeader
								label="Target"
								sortField="target"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[90px]"
							/>
							<TracewayTableHeader
								label="Window"
								sortField="window"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[90px]"
							/>
							<TracewayTableHeader
								label="Current SLI"
								tooltip="Share of good events over the whole window"
								sortField="current_sli"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[120px]"
							/>
							<TracewayTableHeader
								label="Budget left"
								tooltip="Share of the error budget not yet spent; negative once the target is missed"
								sortField="budget"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[170px]"
							/>
							<TracewayTableHeader
								label="Burn 1h"
								tooltip="How fast the budget burned over the last hour; 1x spends it exactly over the window"
								sortField="burn_1h"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								class="w-[100px]"
							/>
							<TracewayTableHeader
								label="Status"
								sortField="status"
								currentSortField={orderBy}
								{sortDirection}
								onSort={(field) => handleSort(field as SortField)}
								align="right"
								class="w-[120px]"
							/>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each visibleSlos as slo (slo.id)}
							{@const tone = budgetTone(slo.budgetRemaining)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(resolve(`/slos/${slo.id}`))}
							>
								<Table.Cell class="font-medium whitespace-normal">
									{slo.name}
									{#if slo.description}
										<div
											class="mt-0.5 max-w-[420px] truncate text-xs font-normal text-muted-foreground"
										>
											{slo.description}
										</div>
									{/if}
									<div
										class="mt-0.5 max-w-[420px] truncate font-mono text-xs font-normal text-muted-foreground"
									>
										{sliSummary(slo)}
									</div>
								</Table.Cell>
								<Table.Cell>
									<SloKindBadge kind={slo.sliKind} />
								</Table.Cell>
								<Table.Cell class="font-mono text-sm tabular-nums">{slo.targetPercent}%</Table.Cell>
								<Table.Cell class="text-sm text-muted-foreground">{slo.windowDays}d</Table.Cell>
								<Table.Cell class="font-mono text-sm tabular-nums {toneTextClass(tone)}">
									{formatSloPercent(slo.currentSli, slo.targetPercent)}
								</Table.Cell>
								<Table.Cell>
									{#if slo.budgetRemaining === null}
										<span class="text-muted-foreground">—</span>
									{:else}
										<div class="flex items-center gap-2">
											<span
												class="w-16 shrink-0 font-mono text-sm tabular-nums {toneTextClass(tone)}"
											>
												{formatBudgetRemaining(slo.budgetRemaining)}
											</span>
											<div class="h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-muted">
												{#if slo.budgetRemaining > 0}
													<div
														class="h-full rounded-full {toneBarClass(tone)}"
														style="width: {Math.max(Math.min(slo.budgetRemaining, 100), 4)}%"
													></div>
												{/if}
											</div>
										</div>
									{/if}
								</Table.Cell>
								<Table.Cell
									class="font-mono text-sm tabular-nums {toneTextClass(burnTone(slo.burnRate1h))}"
								>
									{formatBurnRate(slo.burnRate1h)}
								</Table.Cell>
								<Table.Cell class="text-right">
									<SloStatusBadge status={slo.status} enabled={slo.enabled} />
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				{/if}
			</Table.Root>
		</TableContainer>
	{/if}
</div>

<SloSheet bind:open={sheetOpen} slo={null} onSaved={() => loadData(false)} />
