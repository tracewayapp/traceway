<script lang="ts">
	import { api } from '$lib/api';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import * as Table from '$lib/components/ui/table';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { createRowClickHandler } from '$lib/utils/navigation';

	interface OrgIssueRow {
		exceptionHash: string;
		stackTrace: string;
		lastSeen: string;
		firstSeen: string;
		count: number;
		projectId: string;
		projectName: string;
	}

	let { organizationId }: { organizationId: number } = $props();

	let issues = $state<OrgIssueRow[]>([]);
	let totalGroups = $state(0);
	let loading = $state(true);
	let error = $state('');
	let loadGeneration = 0;

	$effect(() => {
		const orgId = organizationId;
		const generation = ++loadGeneration;
		loading = true;
		error = '';
		api
			.get(`/organizations/${orgId}/overview/issues`)
			.then((response) => {
				if (generation !== loadGeneration) return;
				issues = response.issues || [];
				totalGroups = response.totalGroups || 0;
			})
			.catch((cause) => {
				if (generation !== loadGeneration) return;
				error = getErrorMessage(cause) || 'Failed to load issues';
			})
			.finally(() => {
				if (generation === loadGeneration) loading = false;
			});
	});

	function issueTitle(row: OrgIssueRow): { type: string; message: string } {
		const firstLine = row.stackTrace.split('\n')[0];
		const colonIndex = firstLine.indexOf(':');
		return {
			type: colonIndex > 0 ? firstLine.slice(0, colonIndex) : firstLine,
			message: colonIndex > 0 ? firstLine.slice(colonIndex + 1).trim() : ''
		};
	}
</script>

<section class="space-y-3">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<h2 class="text-lg font-semibold">Issues in the last 24 hours</h2>
		{#if !loading && !error}
			<span class="text-sm text-muted-foreground">
				{totalGroups.toLocaleString()}
				{totalGroups === 1 ? 'issue' : 'issues'} across the organization
			</span>
		{/if}
	</div>

	{#if loading}
		<div class="flex h-48 items-center justify-center">
			<LoadingCircle size="xlg" />
		</div>
	{:else if error}
		<div class="rounded-md border py-16 text-center text-red-500">{error}</div>
	{:else if issues.length === 0}
		<div class="rounded-md border py-16 text-center text-sm text-muted-foreground">
			No issues in the last 24 hours. All quiet.
		</div>
	{:else}
		<TableContainer minWidth="720px">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Issue" />
						<TracewayTableHeader label="Project" class="w-[180px]" />
						<TracewayTableHeader label="Events" class="w-[90px]" align="right" />
						<TracewayTableHeader label="Last seen" class="w-[150px]" align="right" />
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each issues as issue (issue.projectId + issue.exceptionHash)}
						{@const title = issueTitle(issue)}
						<Table.Row
							class="group cursor-pointer"
							onclick={createRowClickHandler(
								`/issues/${issue.exceptionHash}?projectId=${issue.projectId}`
							)}
						>
							<Table.Cell class="max-w-[520px] py-3">
								<div class="min-w-0">
									<div
										class="truncate text-[15px]/6 font-semibold text-foreground group-hover:text-primary dark:group-hover:text-blue-300"
									>
										{title.type}
									</div>
									{#if title.message}
										<div class="truncate text-sm text-muted-foreground">{title.message}</div>
									{/if}
								</div>
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">{issue.projectName}</Table.Cell>
							<Table.Cell class="text-right font-semibold tabular-nums">
								{issue.count.toLocaleString()}
							</Table.Cell>
							<Table.Cell class="text-right text-sm text-muted-foreground">
								{formatRelativeTimeAgo(issue.lastSeen)}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</TableContainer>
		{#if totalGroups > issues.length}
			<p class="text-right text-xs text-muted-foreground">
				Showing the {issues.length} most recently active issues.
			</p>
		{/if}
	{/if}
</section>
