<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { Check, CheckCheck } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { formatDateTime, formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import { type OncallPage } from '$lib/state/oncall.svelte';
	import { runPageAction } from './page-actions';
	import PageBadges from './page-badges.svelte';
	import PageDetailSheet from './page-detail-sheet.svelte';
	import ResolvePageDialog from './resolve-page-dialog.svelte';
	import AcknowledgePagesDialog from './acknowledge-pages-dialog.svelte';

	interface Props {
		deepLinkPageId?: number | null;
	}

	let { deepLinkPageId = null }: Props = $props();

	const filters = [
		{ value: 'active', label: 'All active' },
		{ value: 'open', label: 'Open' },
		{ value: 'acknowledged', label: 'Acknowledged' },
		{ value: 'resolved', label: 'Resolved' }
	];

	const emptyMessages: Record<string, string> = {
		active: 'No active pages. All quiet.',
		open: 'No open pages.',
		acknowledged: 'No acknowledged pages.',
		resolved: 'No resolved pages yet.'
	};

	let statusFilter = $state('active');
	let pages = $state<OncallPage[]>([]);
	let loading = $state(true);
	// "All quiet" and "we could not load your incidents" must never look the
	// same on the incident queue.
	let error = $state('');
	let currentPage = $state(1);
	let pageSize = $state(25);
	let total = $state(0);
	let totalPages = $state(0);

	let sheetOpen = $state(false);
	let selectedPageId = $state<number | null>(null);

	// Bulk selection: resolved pages have no actions, so the resolved tab has
	// no checkboxes at all.
	let selectedIds = $state<Set<number>>(new Set());
	const selectable = $derived(statusFilter !== 'resolved');
	const selectedCount = $derived(selectedIds.size);
	const allSelected = $derived(pages.length > 0 && selectedIds.size === pages.length);
	const someSelected = $derived(selectedIds.size > 0 && selectedIds.size < pages.length);
	const selectedPages = $derived(pages.filter((p) => selectedIds.has(p.id)));
	const selectedOpenPages = $derived(selectedPages.filter((p) => p.status === 'open'));

	// Columns vary by tab: active/open show Status + Actions, acknowledged
	// swaps Status for who/when acknowledged, resolved drops Actions and shows
	// both who/when columns.
	const showStatus = $derived(statusFilter === 'active' || statusFilter === 'open');
	const showAcknowledged = $derived(
		statusFilter === 'acknowledged' || statusFilter === 'resolved'
	);
	const showResolved = $derived(statusFilter === 'resolved');
	const showActions = $derived(statusFilter !== 'resolved');
	const colCount = $derived(selectable ? 8 : 7);

	function ackLabel(item: OncallPage): string {
		if (item.acknowledgedBy !== null) {
			const name = item.acknowledgedByName || `user #${item.acknowledgedBy}`;
			return item.acknowledgedVia === 'link' ? `${name} (via link)` : name;
		}
		return 'Via ack link';
	}

	function resolvedLabel(item: OncallPage): string {
		if (item.resolvedByName) return item.resolvedByName;
		return item.resolvedBy !== null ? `user #${item.resolvedBy}` : '—';
	}

	let loadSeq = 0;

	async function loadPages() {
		const seq = ++loadSeq;
		loading = true;
		error = '';
		try {
			const res = await api.post(
				'/pages',
				{
					status: statusFilter,
					pagination: { page: currentPage, pageSize }
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			if (seq !== loadSeq) return;
			pages = res.data || [];
			total = res.pagination?.total || 0;
			totalPages = res.pagination?.totalPages || 0;
			selectedIds = new Set();
		} catch (e: unknown) {
			if (seq !== loadSeq) return;
			error = e instanceof Error ? e.message : 'Failed to load pages';
			pages = [];
			total = 0;
			totalPages = 0;
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function setFilter(value: string) {
		statusFilter = value;
		currentPage = 1;
		loadPages();
	}

	function handlePageChange(newPage: number) {
		currentPage = newPage;
		loadPages();
	}

	function handlePageSizeChange(newSize: number) {
		pageSize = newSize;
		currentPage = 1;
		loadPages();
	}

	function levelLabel(item: OncallPage): string {
		const stepCount = item.policySnapshot?.steps?.length ?? 0;
		if (item.escalationLevel < 0) {
			// -1 = "next escalation starts at level 0", also how a page waiting
			// for its next repeat is stored.
			if (item.repeatIteration > 0) {
				return `L${stepCount} of ${stepCount} · repeat ${item.repeatIteration}`;
			}
			return '—';
		}
		let label = `L${item.escalationLevel + 1} of ${stepCount}`;
		if (item.status === 'open' && item.nextEscalationAt === null) {
			label += ' · exhausted';
		}
		return label;
	}

	async function acknowledge(item: OncallPage, e: Event) {
		e.stopPropagation();
		await runPageAction(item.id, 'acknowledge');
		loadPages();
	}

	let resolveDialogOpen = $state(false);
	let resolveTargets = $state<OncallPage[]>([]);
	let ackDialogOpen = $state(false);

	function resolve(item: OncallPage, e: Event) {
		e.stopPropagation();
		resolveTargets = [item];
		resolveDialogOpen = true;
	}

	function openBulkResolve() {
		resolveTargets = selectedPages;
		resolveDialogOpen = true;
	}

	// Selection handlers
	function toggleSelectAll() {
		if (allSelected) {
			selectedIds = new Set();
		} else {
			selectedIds = new Set(pages.map((p) => p.id));
		}
	}

	function toggleSelect(id: number) {
		const newSet = new Set(selectedIds);
		if (newSet.has(id)) {
			newSet.delete(id);
		} else {
			newSet.add(id);
		}
		selectedIds = newSet;
	}

	function isSelected(id: number): boolean {
		return selectedIds.has(id);
	}

	function openDetail(item: OncallPage) {
		selectedPageId = item.id;
		sheetOpen = true;
	}

	onMount(() => {
		loadPages();
		if (deepLinkPageId !== null) {
			selectedPageId = deepLinkPageId;
			sheetOpen = true;
		}
	});
</script>

<div class="space-y-4">
	<div class="flex flex-wrap items-center gap-2">
		{#each filters as filter (filter.value)}
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

	<!-- Bulk actions toolbar - shown when items selected -->
	{#if selectable && selectedCount > 0}
		<div
			class="flex animate-in items-center gap-3 rounded-md border bg-muted/50 p-3 duration-200 fade-in slide-in-from-top-1"
		>
			<span class="text-sm font-medium"
				>{selectedCount} page{selectedCount === 1 ? '' : 's'} selected</span
			>
			<Button
				variant="outline"
				size="sm"
				onclick={() => (ackDialogOpen = true)}
				disabled={selectedOpenPages.length === 0}
				class="gap-1.5"
			>
				<Check class="h-4 w-4" />
				Acknowledge
			</Button>
			<Button variant="outline" size="sm" onclick={openBulkResolve} class="gap-1.5">
				<CheckCheck class="h-4 w-4" />
				Resolve
			</Button>
			<Button variant="ghost" size="sm" onclick={() => (selectedIds = new Set())}>
				Clear selection
			</Button>
		</div>
	{/if}

	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={colCount} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={colCount} class="h-48">
							<div class="flex h-full flex-col items-center justify-center gap-3">
								<p class="text-sm text-destructive">{error}</p>
								<Button variant="outline" size="sm" onclick={() => loadPages()}>Retry</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if pages.length === 0}
				<Table.Body>
					<TableEmptyState colspan={colCount} message={emptyMessages[statusFilter]} />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						{#if selectable}
							<Table.Head class="w-[40px] pl-4">
								<Checkbox
									checked={allSelected ? true : someSelected ? 'indeterminate' : false}
									onCheckedChange={toggleSelectAll}
									aria-label="Select all"
								/>
							</Table.Head>
						{/if}
						<Table.Head>Severity</Table.Head>
						<Table.Head>Subject</Table.Head>
						<Table.Head>Level</Table.Head>
						<Table.Head>Age</Table.Head>
						<Table.Head>Events</Table.Head>
						{#if showStatus}
							<Table.Head>Status</Table.Head>
						{/if}
						{#if showAcknowledged}
							<Table.Head>Acknowledged</Table.Head>
						{/if}
						{#if showResolved}
							<Table.Head>Resolved</Table.Head>
						{/if}
						{#if showActions}
							<Table.Head class="text-right">Actions</Table.Head>
						{/if}
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pages as item (item.id)}
						<Table.Row
							class="cursor-pointer"
							data-state={selectable && isSelected(item.id) ? 'selected' : undefined}
							onclick={() => openDetail(item)}
						>
							{#if selectable}
								<Table.Cell class="pl-4" onclick={(e) => e.stopPropagation()}>
									<Checkbox
										checked={isSelected(item.id)}
										onCheckedChange={() => toggleSelect(item.id)}
										aria-label="Select row"
									/>
								</Table.Cell>
							{/if}
							<Table.Cell>
								<div class="flex items-center gap-1">
									<PageBadges severity={item.severity} urgency={item.urgency} fallback />
								</div>
							</Table.Cell>
							<Table.Cell class="max-w-xs">
								<div class="truncate font-medium">{item.subject || '—'}</div>
								{#if item.ruleName}
									<div class="truncate text-xs text-muted-foreground">{item.ruleName}</div>
								{/if}
							</Table.Cell>
							<Table.Cell class="whitespace-nowrap">{levelLabel(item)}</Table.Cell>
							<Table.Cell class="whitespace-nowrap text-muted-foreground"
								>{formatRelativeTimeAgo(item.createdAt)}</Table.Cell
							>
							<Table.Cell>{item.eventCount}</Table.Cell>
							{#if showStatus}
								<Table.Cell>
									<PageBadges status={item.status} />
								</Table.Cell>
							{/if}
							{#if showAcknowledged}
								<Table.Cell class="whitespace-nowrap">
									{#if item.acknowledgedAt}
										<div>{ackLabel(item)}</div>
										<div class="text-xs text-muted-foreground">
											{formatDateTime(item.acknowledgedAt, { format: 'short' })}
										</div>
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</Table.Cell>
							{/if}
							{#if showResolved}
								<Table.Cell class="whitespace-nowrap">
									{#if item.resolvedAt}
										<div>{resolvedLabel(item)}</div>
										<div class="text-xs text-muted-foreground">
											{formatDateTime(item.resolvedAt, { format: 'short' })}
										</div>
									{:else}
										<span class="text-muted-foreground">—</span>
									{/if}
								</Table.Cell>
							{/if}
							{#if showActions}
								<Table.Cell class="text-right">
									<div class="flex justify-end gap-1">
										{#if item.status === 'open'}
											<Button
												variant="ghost"
												size="icon"
												title="Acknowledge"
												onclick={(e) => acknowledge(item, e)}
											>
												<Check class="h-4 w-4" />
											</Button>
										{/if}
										{#if item.status !== 'resolved'}
											<Button
												variant="ghost"
												size="icon"
												title="Resolve"
												onclick={(e) => resolve(item, e)}
											>
												<CheckCheck class="h-4 w-4" />
											</Button>
										{/if}
									</div>
								</Table.Cell>
							{/if}
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<PaginationFooter
		{currentPage}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		itemLabel="page"
	/>
</div>

<PageDetailSheet bind:open={sheetOpen} pageId={selectedPageId} onChanged={loadPages} />
<ResolvePageDialog bind:open={resolveDialogOpen} pages={resolveTargets} onResolved={loadPages} />
<AcknowledgePagesDialog bind:open={ackDialogOpen} pages={selectedOpenPages} onAcknowledged={loadPages} />
