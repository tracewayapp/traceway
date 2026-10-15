<script lang="ts">
	import { untrack } from 'svelte';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Switch } from '$lib/components/ui/switch';
	import { Label } from '$lib/components/ui/label';
	import * as Table from '$lib/components/ui/table';
	import { Badge } from '$lib/components/ui/badge';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import { toast } from 'svelte-sonner';
	import { formatDateTime } from '$lib/utils/formatters';
	import { organizationState, type AgentWriteEvent } from '$lib/state/organization.svelte';

	let { organizationId }: { organizationId: number } = $props();

	let saving = $state(false);
	// Overridden while a change is saving, and set back if it fails, so the
	// switch never shows a state the server did not accept.
	let writeEnabled = $derived(organizationState.agentWriteEnabled);
	let events = $state<AgentWriteEvent[]>([]);
	let loading = $state(true);
	let error = $state('');
	let page = $state(1);
	let pageSize = $state(20);
	let total = $state(0);
	let totalPages = $state(0);

	async function loadActivity() {
		loading = true;
		error = '';
		try {
			const res = await organizationState.loadAgentActivity(organizationId, page, pageSize);
			events = res.data;
			total = res.pagination.total;
			totalPages = res.pagination.totalPages;
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to load agent activity';
		} finally {
			loading = false;
		}
	}

	// Only a change of organization reloads from page 1; loadActivity reads
	// page and pageSize, which must not become dependencies of this effect.
	$effect(() => {
		if (organizationId) {
			untrack(() => {
				page = 1;
				loadActivity();
			});
		}
	});

	async function toggle(enabled: boolean) {
		saving = true;
		try {
			await organizationState.setAgentWriteEnabled(organizationId, enabled);
			toast.success(enabled ? 'Agent write access turned on' : 'Agent write access turned off', {
				position: 'top-center'
			});
		} catch (e: unknown) {
			writeEnabled = organizationState.agentWriteEnabled;
			toast.error(e instanceof Error ? e.message : 'Failed to update agent access');
		} finally {
			saving = false;
		}
	}

	function describeRoute(event: AgentWriteEvent): string {
		return `${event.method} ${event.path.replace(/^\/api/, '')}`;
	}

	function credentialLabel(event: AgentWriteEvent): string {
		if (event.credentialKind === 'pat') {
			return event.credentialName ? `Token "${event.credentialName}"` : 'Personal access token';
		}
		if (event.credentialRef === 'traceway-cli-write') return 'Write CLI';
		return event.credentialName ? `MCP client "${event.credentialName}"` : 'MCP client';
	}
</script>

<Card>
	<CardHeader>
		<CardTitle>Agent Access</CardTitle>
		<CardDescription>
			Control whether write tokens, the traceway-write CLI and the write MCP server can change data
			in this organization. Read access is always available to members.
		</CardDescription>
	</CardHeader>
	<CardContent class="space-y-6">
		<div class="flex items-start justify-between gap-4">
			<div class="space-y-1">
				<Label for="agent-write-toggle">Allow agent writes</Label>
				<p class="text-sm text-muted-foreground">
					Agents can archive issues, acknowledge and resolve pages, snooze or toggle alert rules,
					run monitors, edit post-mortems and mark slow endpoints. They never get more access than
					the person who created the token.
				</p>
			</div>
			<Switch
				id="agent-write-toggle"
				checked={writeEnabled}
				disabled={saving}
				onCheckedChange={(checked) => {
					writeEnabled = checked;
					toggle(checked);
				}}
			/>
		</div>

		{#if organizationState.agentWriteEnabled}
			<WarningCallout title="Agent writes are on">
				Anyone in this organization with a write token can let an AI agent change shared data. Every
				write is listed below.
			</WarningCallout>
		{/if}

		<div class="space-y-3">
			<h3 class="text-sm font-medium">Agent activity</h3>
			{#if loading}
				<div class="flex justify-center py-8"><LoadingCircle size="lg" /></div>
			{:else if error}
				<ErrorRetryBox message={error} onRetry={loadActivity} />
			{:else if events.length === 0}
				<EmptyState message="No agent writes yet." />
			{:else}
				<TableContainer>
					<Table.Root>
						<Table.Header>
							<Table.Row class="hover:bg-transparent">
								<Table.Head>When</Table.Head>
								<Table.Head>Who</Table.Head>
								<Table.Head>Credential</Table.Head>
								<Table.Head>Project</Table.Head>
								<Table.Head>Action</Table.Head>
								<Table.Head>Result</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each events as event (event.id)}
								<Table.Row>
									<Table.Cell class="whitespace-nowrap"
										>{formatDateTime(event.createdAt, { format: 'short' })}</Table.Cell
									>
									<Table.Cell>{event.userEmail}</Table.Cell>
									<Table.Cell>{credentialLabel(event)}</Table.Cell>
									<Table.Cell>{event.projectName || event.projectId}</Table.Cell>
									<Table.Cell>
										<code class="rounded bg-muted px-1.5 py-0.5 font-mono text-xs"
											>{describeRoute(event)}</code
										>
									</Table.Cell>
									<Table.Cell>
										{#if event.statusCode < 400}
											<Badge variant="outline">{event.statusCode}</Badge>
										{:else}
											<Badge variant="destructive">{event.statusCode}</Badge>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</TableContainer>
				<PaginationFooter
					currentPage={page}
					{totalPages}
					{pageSize}
					totalItems={total}
					onPageChange={(p) => {
						page = p;
						loadActivity();
					}}
					onPageSizeChange={(size) => {
						pageSize = size;
						page = 1;
						loadActivity();
					}}
					{loading}
					itemLabel="write"
				/>
			{/if}
		</div>
	</CardContent>
</Card>
