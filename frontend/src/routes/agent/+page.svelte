<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import RepositoryTab from './repository-tab.svelte';
	import { Check } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { agentState } from '$lib/state/agent.svelte';
	import { setTabParam } from '$lib/utils/url-params';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { getErrorMessage } from '$lib/utils/errors';
	import { attemptTitle, formatCost, statusLabel, statusTone } from '$lib/utils/agent';
	import type { Attempt } from '$lib/types/agent';

	const TABS = [
		{ value: 'attempts', label: 'Attempts' },
		{ value: 'repository', label: 'Repository' }
	];
	const activeTab = $derived(page.url.searchParams.get('tab') || 'attempts');

	const STATUS_FILTERS = [
		{ value: 'all', label: 'All statuses' },
		{ value: 'pending_approval', label: 'Pending approval' },
		{ value: 'queued', label: 'Queued' },
		{ value: 'running', label: 'Running' },
		{ value: 'needs_input', label: 'Needs input' },
		{ value: 'awaiting_review', label: 'Awaiting review' },
		{ value: 'merged', label: 'Merged' },
		{ value: 'analyzed', label: 'Analyzed' },
		{ value: 'failed', label: 'Failed' }
	];

	let attempts = $state<Attempt[]>([]);
	let pending = $state<Attempt[]>([]);
	let loading = $state(true);
	let error = $state('');
	let statusFilter = $state('all');
	let currentPage = $state(1);
	let pageSize = $state(25);
	let total = $state(0);
	let totalPages = $state(0);
	let approving = $state<string | null>(null);

	async function loadAttempts() {
		loading = true;
		error = '';
		try {
			const projectId = projectsState.currentProjectId ?? undefined;
			const [listed, pendingList] = await Promise.all([
				api.post(
					'/agent/attempts/list',
					{
						status: statusFilter === 'all' ? '' : statusFilter,
						pagination: { page: currentPage, pageSize }
					},
					{ projectId }
				),
				api.post(
					'/agent/attempts/list',
					{ status: 'pending_approval', pagination: { page: 1, pageSize: 50 } },
					{ projectId }
				)
			]);
			attempts = listed.data ?? [];
			total = listed.pagination?.total ?? 0;
			totalPages = listed.pagination?.totalPages ?? 0;
			pending = pendingList.data ?? [];
		} catch (e) {
			error = getErrorMessage(e, 'Failed to load attempts');
		} finally {
			loading = false;
		}
	}

	async function approve(attempt: Attempt) {
		approving = attempt.id;
		try {
			await api.post(
				`/agent/attempts/${attempt.id}/approve`,
				{},
				{
					projectId: projectsState.currentProjectId ?? undefined
				}
			);
			toast.success('Successfully approved the Attempt', { position: 'top-center' });
			agentState.refreshBadge();
			await loadAttempts();
		} catch (e) {
			toast.error(getErrorMessage(e, 'Failed to approve the attempt'));
		} finally {
			approving = null;
		}
	}

	function setTab(tab: string) {
		setTabParam(tab);
	}

	onMount(() => {
		loadAttempts();
	});

	$effect(() => {
		void statusFilter;
		void currentPage;
		void pageSize;
		if (activeTab === 'attempts') loadAttempts();
	});
</script>

<div class="space-y-4">
	<PageHeader title="Agent" />

	<PageTabs tabs={TABS} {activeTab} onTabChange={setTab} />

	{#if activeTab === 'repository'}
		<RepositoryTab />
	{:else}
		<InfoCallout>
			Attempts are the agent's runs against an issue. Start one from an issue page with Fix it; a
			pending attempt waits here for approval.
		</InfoCallout>

		{#if pending.length > 0}
			<div
				class="space-y-2 rounded-md border border-amber-500/40 p-4"
				data-testid="pending-approvals"
			>
				<p class="text-sm font-medium">Waiting for approval</p>
				{#each pending as attempt (attempt.id)}
					<div class="flex items-center justify-between gap-3 text-sm">
						<a
							{...{ href: `/agent/${attempt.id}` }}
							class="min-w-0 truncate text-blue-600 hover:underline dark:text-blue-400"
						>
							{attemptTitle(attempt.subjectKind, attempt.subjectRef)} · attempt {attempt.number}
						</a>
						{#if projectsState.canWriteCurrentProject}
							<Button
								size="sm"
								onclick={() => approve(attempt)}
								disabled={approving === attempt.id}
							>
								<Check class="mr-1 h-3 w-3" />
								{approving === attempt.id ? 'Approving...' : 'Approve'}
							</Button>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<div class="flex items-center gap-2" data-testid="status-filter">
			<Select.Root
				type="single"
				value={statusFilter}
				onValueChange={(v) => {
					if (v) {
						statusFilter = v;
						currentPage = 1;
					}
				}}
			>
				<Select.Trigger class="w-48">
					{STATUS_FILTERS.find((f) => f.value === statusFilter)?.label}
				</Select.Trigger>
				<Select.Content>
					{#each STATUS_FILTERS as filter (filter.value)}
						<Select.Item value={filter.value}>{filter.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		{#if loading && attempts.length === 0}
			<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if error}
			<ErrorRetryBox message={error} onRetry={() => loadAttempts()} />
		{:else if attempts.length === 0}
			<EmptyState message="No attempts yet. Open an issue and click Fix it." />
		{:else}
			<TableContainer>
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Subject</Table.Head>
							<Table.Head>#</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head>Agent</Table.Head>
							<Table.Head>Cost</Table.Head>
							<Table.Head>Created</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each attempts as attempt (attempt.id)}
							<Table.Row
								class="cursor-pointer"
								onclick={createRowClickHandler(`/agent/${attempt.id}`)}
							>
								<Table.Cell class="font-mono text-xs">
									{attemptTitle(attempt.subjectKind, attempt.subjectRef)}
								</Table.Cell>
								<Table.Cell>{attempt.number}</Table.Cell>
								<Table.Cell>
									<StatusPill
										tone={statusTone(attempt.status)}
										label={statusLabel(attempt.status)}
									/>
								</Table.Cell>
								<Table.Cell class="text-muted-foreground">{attempt.agent || '-'}</Table.Cell>
								<Table.Cell>{formatCost(attempt.costUsd)}</Table.Cell>
								<Table.Cell class="text-muted-foreground">
									{new Date(attempt.createdAt).toLocaleString()}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
			<PaginationFooter
				{currentPage}
				{totalPages}
				{pageSize}
				totalItems={total}
				onPageChange={(p) => (currentPage = p)}
				onPageSizeChange={(size) => {
					pageSize = size;
					currentPage = 1;
				}}
				{loading}
				itemLabel="attempt"
			/>
		{/if}
	{/if}
</div>
