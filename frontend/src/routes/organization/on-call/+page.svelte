<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import { organizationContext } from '$lib/state/organization-context.svelte';
	import type { OncallPage } from '$lib/state/oncall.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatDateTime, formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { TimeRangeState } from '$lib/utils/time-range-state.svelte';
	import { updateUrl } from '$lib/utils/url-params';
	import * as Table from '$lib/components/ui/table';
	import { Button } from '$lib/components/ui/button';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import PageBadges from '$lib/components/traceway/page-badges.svelte';
	import CountPill from '../count-pill.svelte';

	interface OrgPageRow extends OncallPage {
		projectName: string;
	}

	const REFRESH_MS = 60 * 1000;

	const statusFilters = [
		{ value: 'active', label: 'All active' },
		{ value: 'open', label: 'Open' },
		{ value: 'acknowledged', label: 'Acknowledged' },
		{ value: 'resolved', label: 'Resolved' }
	];
	const statusValues = new Set(statusFilters.map((filter) => filter.value));

	const emptyMessages: Record<string, string> = {
		active: 'No active pages opened in the selected range. All quiet.',
		open: 'No open pages in the selected range.',
		acknowledged: 'No acknowledged pages in the selected range.',
		resolved: 'No resolved pages in the selected range.'
	};

	const range = new TimeRangeState('1M');

	function readParams() {
		if (!browser) return { status: 'active', search: '' };
		const params = new URLSearchParams(window.location.search);
		const status = params.get('status') || 'active';
		return {
			status: statusValues.has(status) ? status : 'active',
			search: params.get('search') || ''
		};
	}

	const initialParams = readParams();
	let statusFilter = $state(initialParams.status);
	let searchQuery = $state(initialParams.search);

	let pages = $state<OrgPageRow[]>([]);
	let openPagesCount = $state(0);
	let loading = $state(true);
	let error = $state('');
	let currentPage = $state(1);
	let pageSize = $state(25);
	let total = $state(0);
	let totalPages = $state(0);
	let loadGeneration = 0;
	let activeOrganizationId: number | null = null;

	function syncUrl(pushToHistory: boolean) {
		updateUrl(
			{
				...range.urlParams(),
				status: statusFilter !== 'active' ? statusFilter : null,
				search: searchQuery.trim() || null
			},
			{ pushToHistory }
		);
	}

	async function loadPages(pushToHistory: boolean, background = false) {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null) return;
		const generation = ++loadGeneration;
		if (!background) loading = true;
		error = '';
		range.resolvePreset();
		syncUrl(pushToHistory);
		try {
			const response = await api.post(`/organizations/${organizationId}/overview/pages`, {
				status: statusFilter,
				search: searchQuery.trim(),
				fromDate: range.fromUTC(),
				toDate: range.toUTC(),
				pagination: { page: currentPage, pageSize }
			});
			if (generation !== loadGeneration) return;
			pages = response.data || [];
			total = response.pagination?.total || 0;
			totalPages = response.pagination?.totalPages || 0;
			openPagesCount = response.openPagesCount || 0;
		} catch (e) {
			if (generation !== loadGeneration) return;
			error = getErrorMessage(e) || 'Failed to load on-call pages';
		} finally {
			if (generation === loadGeneration) loading = false;
		}
	}

	function setFilter(value: string) {
		statusFilter = value;
		currentPage = 1;
		loadPages(true);
	}

	function handleSearch() {
		currentPage = 1;
		loadPages(true);
	}

	function handleTimeRangeChange(
		from: Parameters<TimeRangeState['apply']>[0],
		to: Parameters<TimeRangeState['apply']>[1],
		preset: string | null
	) {
		range.apply(from, to, preset);
		currentPage = 1;
		loadPages(true);
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			currentPage = newPage;
			loadPages(true);
		}
	}

	function handlePageSizeChange(newSize: number) {
		pageSize = newSize;
		currentPage = 1;
		loadPages(true);
	}

	function handlePopState() {
		range.readUrl();
		const params = readParams();
		statusFilter = params.status;
		searchQuery = params.search;
		currentPage = 1;
		loadPages(false);
	}

	$effect(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null) return;
		if (organizationId !== activeOrganizationId) {
			activeOrganizationId = organizationId;
			untrack(() => {
				currentPage = 1;
				loadPages(false);
			});
		}
		const timer = window.setInterval(() => void loadPages(false, true), REFRESH_MS);
		return () => window.clearInterval(timer);
	});

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<PageHeader
		title="On-Call"
		description="Pages from every project, filtered by when they were opened. Acknowledged pages stay active until they are resolved."
	>
		{#snippet trailing()}
			{#if !loading && !error}
				<CountPill count={total} />
				{#if openPagesCount > 0}
					<CountPill count={openPagesCount} label="open" tone="danger" />
				{/if}
			{/if}
		{/snippet}
		{#snippet actions()}
			<TimeRangePicker
				bind:fromDate={range.fromDate}
				bind:toDate={range.toDate}
				bind:fromTime={range.fromTime}
				bind:toTime={range.toTime}
				bind:preset={range.preset}
				onApply={handleTimeRangeChange}
			/>
		{/snippet}
	</PageHeader>

	<SearchBar
		placeholder="Search pages by subject or rule..."
		bind:value={searchQuery}
		onSearch={handleSearch}
		disabled={loading}
	/>

	<div class="flex flex-wrap items-center gap-2">
		{#each statusFilters as filter (filter.value)}
			<Button
				size="sm"
				variant={statusFilter === filter.value ? 'default' : 'outline'}
				class="rounded-full"
				onclick={() => setFilter(filter.value)}
			>
				{filter.label}
			</Button>
		{/each}
	</div>

	<TableContainer minWidth="820px">
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
						<Table.Cell colspan={5} class="h-48">
							<div class="flex h-full flex-col items-center justify-center gap-3">
								<p class="text-sm text-destructive">{error}</p>
								<Button variant="outline" size="sm" onclick={() => loadPages(false)}>Retry</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if pages.length === 0}
				<Table.Body>
					<TableEmptyState colspan={5} message={emptyMessages[statusFilter]} />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Page" />
						<TracewayTableHeader label="Project" class="w-[180px]" />
						<TracewayTableHeader label="Severity" class="w-[160px]" />
						<TracewayTableHeader label="Status" class="w-[130px]" />
						<TracewayTableHeader label="Opened" class="w-[170px]" align="right" />
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pages as oncallPage (oncallPage.id)}
						<Table.Row
							class="cursor-pointer"
							onclick={createRowClickHandler(
								`/on-call?tab=pages&page=${oncallPage.id}&projectId=${oncallPage.projectId}`
							)}
						>
							<Table.Cell class="max-w-[420px] py-3">
								<div class="min-w-0">
									<div class="truncate font-medium">{oncallPage.subject || '—'}</div>
									{#if oncallPage.ruleName}
										<div class="truncate text-xs text-muted-foreground">
											{oncallPage.ruleName}
										</div>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{oncallPage.projectName || '—'}
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-wrap gap-1">
									<PageBadges
										severity={oncallPage.severity}
										urgency={oncallPage.urgency}
										fallback
									/>
								</div>
							</Table.Cell>
							<Table.Cell><PageBadges status={oncallPage.status} /></Table.Cell>
							<Table.Cell
								class="text-right text-sm text-muted-foreground"
								title={formatDateTime(oncallPage.createdAt)}
							>
								{formatRelativeTimeAgo(oncallPage.createdAt)}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</TableContainer>

	<PaginationFooter
		{currentPage}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		{loading}
		itemLabel="page"
	/>
</div>
