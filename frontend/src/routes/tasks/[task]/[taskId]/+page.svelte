<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { formatDuration, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import { ArrowRight, TriangleAlert, ClipboardList } from 'lucide-svelte';
	import { LabelValue } from '$lib/components/ui/label-value';
	import { ContextGrid } from '$lib/components/ui/context-grid';
	import SegmentWaterfall from '$lib/components/segments/segment-waterfall.svelte';
	import SegmentEmptyState from '$lib/components/segments/segment-empty-state.svelte';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';

	type TaskDetailResponse = {
		task: {
			id: string;
			taskName: string;
			duration: number;
			recordedAt: string;
			clientIP: string;
			scope: Record<string, string> | null;
			serverName: string;
			appVersion: string;
		};
		exception?: {
			exceptionHash: string;
			stackTrace: string;
		};
		messages: {
			id: string;
			exceptionHash: string;
			stackTrace: string;
			recordedAt: string;
			scope?: Record<string, string>;
		}[];
		segments: any[];
		hasSegments: boolean;
	};

	let { data } = $props();

	const timezone = $derived(getTimezone());

	let response = $state<TaskDetailResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const result = await api.post(
				`/tasks/${data.taskId}`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			response = result;
		} catch (e: unknown) {
			console.error(e);
			const err = e as { status?: number; message?: string };
			if (err.status === 404) {
				notFound = true;
			} else {
				error = err.message || 'Failed to load task details';
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<PageHeader
		title={decodeURIComponent(data.task)} subtitle={`Task ID: ${data.taskId}`}
		onBack={createRowClickHandler(resolve('/tasks/[task]', {task: encodeURIComponent(data.task)}))} />


	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Task Not Found"
			description="The task instance you're looking for doesn't exist or may have expired."
			onBack={createRowClickHandler(resolve('/tasks/[task]', {task: encodeURIComponent(data.task)}))}
			backLabel="Back to Task"
			onRetry={loadData}
			identifier={data.taskId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load Task"
			description={error}
			onBack={createRowClickHandler(resolve('/tasks/[task]', {task: encodeURIComponent(data.task)}))}
			backLabel="Back to Task"
			onRetry={loadData}
		/>
	{:else if response}
		<!-- Task Details Card -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Task Details</Card.Title>
				<Card.Description>Details of this specific task execution</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-6">
				<div class="grid grid-cols-2 gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
					<LabelValue
						label="Task"
						value={decodeURIComponent(data.task)}
						mono
					/>
					<LabelValue
						label="Duration"
						value={formatDuration(response.task.duration)}
						mono
						large
					/>
					<LabelValue
						label="Recorded At"
						value={formatDateTime(response.task.recordedAt, { timezone })}
						mono
					/>
					<LabelValue
						label="Server"
						value={response.task.serverName}
						mono
					/>
					<LabelValue
						label="Version"
						value={response.task.appVersion || '-'}
						mono
					/>
				</div>

				{#if response.task.scope && Object.keys(response.task.scope).length > 0}
					<hr class="border-border" />
					<div>
						<p class="mb-3 text-sm font-medium">Context (Scope)</p>
						<ContextGrid scope={response.task.scope} />
					</div>
				{/if}
			</Card.Content>
		</Card.Root>

		<!-- Exception Card (if exception exists) -->
		{#if response.exception}
			<Card.Root class="border-red-500/30 bg-red-500/5">
				<Card.Header>
					<div class="flex items-center gap-2">
						<TriangleAlert class="h-5 w-5 text-red-500" />
						<Card.Title class="text-red-600 dark:text-red-400">Exception Occurred</Card.Title>
					</div>
					<Card.Description>This task execution resulted in an exception</Card.Description>
				</Card.Header>
				<Card.Content>
					<div class="mb-4 max-h-32 overflow-x-auto rounded-md bg-muted p-3">
						<pre class="font-mono text-sm whitespace-pre-wrap">{response.exception.stackTrace
								.split('\n')
								.slice(0, 4)
								.join('\n')}{response.exception.stackTrace.split('\n').length > 4
								? '\n...'
								: ''}</pre>
					</div>
					<Button
						variant="outline"
						size="sm"
						onclick={() => goto(`/issues/${response!.exception!.exceptionHash}`)}
					>
						View Full Exception
						<ArrowRight class="ml-2 h-4 w-4" />
					</Button>
				</Card.Content>
			</Card.Root>
		{/if}

		<!-- Messages Section (if messages exist) -->
		{#if response.messages.length > 0}
			<Card.Root>
				<Card.Header>
					<div class="flex items-center gap-2">
						<ClipboardList class="h-5 w-5 text-muted-foreground" />
						<Card.Title>Messages</Card.Title>
					</div>
					<Card.Description>
						{response.messages.length} message{response.messages.length === 1 ? '' : 's'} logged during this task
					</Card.Description>
				</Card.Header>
				<Card.Content class="px-0 pb-0">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="pl-6">Message</Table.Head>
								<Table.Head class="w-[180px]">Recorded At</Table.Head>
								<Table.Head class="w-[100px] pr-6">Scope</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each response.messages as message}
								<Table.Row
									class="cursor-pointer hover:bg-muted/50"
									onclick={() => goto(`/issues/${message.exceptionHash}/${message.id}`)}
								>
									<Table.Cell class="pl-6">
										<div class="max-w-md truncate font-mono text-sm">
											{message.stackTrace.split('\n')[0]}
										</div>
									</Table.Cell>
									<Table.Cell class="font-mono text-sm text-muted-foreground">
										{formatDateTime(message.recordedAt, { timezone })}
									</Table.Cell>
									<Table.Cell class="pr-6">
										{#if message.scope && Object.keys(message.scope).length > 0}
											<span class="text-xs text-muted-foreground">
												{Object.keys(message.scope).length} key{Object.keys(message.scope).length === 1 ? '' : 's'}
											</span>
										{:else}
											<span class="text-xs text-muted-foreground">-</span>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</Card.Content>
			</Card.Root>
		{/if}

		<!-- Segments Section -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Segments</Card.Title>
				<Card.Description>
					{#if response.hasSegments}
						Timing breakdown of operations within this task
					{:else}
						No segments recorded for this task
					{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				{#if response.hasSegments}
					<SegmentWaterfall
						segments={response.segments}
						transactionDuration={response.task.duration}
						transactionStartTime={response.task.recordedAt}
					/>
				{:else}
					<SegmentEmptyState framework={projectsState.currentProject?.framework ?? 'gin'} />
				{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</div>
