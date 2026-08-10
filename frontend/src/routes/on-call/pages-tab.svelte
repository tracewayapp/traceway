<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { Check, CheckCheck } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { projectsState } from '$lib/state/projects.svelte';
	import { type OncallPage } from '$lib/state/oncall.svelte';
	import { runPageAction } from './page-actions';
	import PageBadges from './page-badges.svelte';
	import PageDetailSheet from './page-detail-sheet.svelte';

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
		if (item.escalationLevel < 0) return '—';
		const stepCount = item.policySnapshot?.steps?.length ?? 0;
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

	async function resolve(item: OncallPage, e: Event) {
		e.stopPropagation();
		await runPageAction(item.id, 'resolve');
		loadPages();
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

	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={7} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={7} class="h-48">
							<div class="flex h-full flex-col items-center justify-center gap-3">
								<p class="text-sm text-destructive">{error}</p>
								<Button variant="outline" size="sm" onclick={() => loadPages()}>Retry</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if pages.length === 0}
				<Table.Body>
					<TableEmptyState colspan={7} message={emptyMessages[statusFilter]} />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<Table.Head>Severity</Table.Head>
						<Table.Head>Subject</Table.Head>
						<Table.Head>Level</Table.Head>
						<Table.Head>Age</Table.Head>
						<Table.Head>Events</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head class="text-right">Actions</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each pages as item (item.id)}
						<Table.Row class="cursor-pointer" onclick={() => openDetail(item)}>
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
							<Table.Cell>
								<PageBadges status={item.status} />
							</Table.Cell>
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
