<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import { ArrowLeft, ArrowRight, TriangleAlert } from 'lucide-svelte';
	import { LabelValue } from '$lib/components/ui/label-value';
	import { ContextGrid } from '$lib/components/ui/context-grid';
	import SegmentWaterfall from '$lib/components/segments/segment-waterfall.svelte';
	import SegmentEmptyState from '$lib/components/segments/segment-empty-state.svelte';
	import type { TransactionDetailResponse } from '$lib/types/segments';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';

	let { data } = $props();

	const timezone = $derived(getTimezone());

	let response = $state<TransactionDetailResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notFound = $state(false);

	async function loadData() {
		loading = true;
		error = '';
		notFound = false;

		try {
			const result = await api.post(
				`/transactions/${data.transactionId}`,
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
				error = err.message || 'Failed to load transaction details';
			}
		} finally {
			loading = false;
		}
	}

	function formatBytes(bytes: number): string {
		if (bytes < 1024) {
			return `${bytes} B`;
		} else if (bytes < 1024 * 1024) {
			return `${(bytes / 1024).toFixed(1)} KB`;
		} else {
			return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
		}
	}

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<PageHeader
		title={decodeURIComponent(data.endpoint)} subtitle={`Transaction ID: ${data.transactionId}`}
		onBack={createRowClickHandler(resolve('/transactions/[endpoint]', {endpoint: encodeURIComponent(data.endpoint)}))} />


	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Transaction Not Found"
			description="The transaction you're looking for doesn't exist or may have expired."
			onBack={createRowClickHandler(resolve('/transactions/[endpoint]', {endpoint: encodeURIComponent(data.endpoint)}))}
			backLabel="Back to Endpoint"
			onRetry={loadData}
			identifier={data.transactionId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load Transaction"
			description={error}
			onBack={createRowClickHandler(resolve('/transactions/[endpoint]', {endpoint: encodeURIComponent(data.endpoint)}))}
			backLabel="Back to Endpoint"
			onRetry={loadData}
		/>
	{:else if response}
		<!-- Transaction Details Card -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Transaction Details</Card.Title>
				<Card.Description>Details of this specific transaction occurrence</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-6">
				<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
					<LabelValue
						label="Endpoint"
						value={decodeURIComponent(data.endpoint)}
						mono
					/>
					<LabelValue
						label="Status"
						value={response.transaction.statusCode}
						mono
						large
						valueClass={getStatusColor(response.transaction.statusCode)}
					/>
					<LabelValue
						label="Duration"
						value={formatDuration(response.transaction.duration)}
						mono
						large
					/>
					<LabelValue
						label="Recorded At"
						value={formatDateTime(response.transaction.recordedAt, { timezone })}
						mono
					/>
					<LabelValue
						label="Server"
						value={response.transaction.serverName}
						mono
					/>
					<LabelValue
						label="Version"
						value={response.transaction.appVersion}
						mono
					/>
					<LabelValue
						label="Client IP"
						value={response.transaction.clientIP}
						mono
					/>
					<LabelValue
						label="Body Size"
						value={formatBytes(response.transaction.bodySize)}
						mono
					/>
				</div>

				{#if response.transaction.scope && Object.keys(response.transaction.scope).length > 0}
					<hr class="border-border" />
					<div>
						<p class="mb-3 text-sm font-medium">Context (Scope)</p>
						<ContextGrid scope={response.transaction.scope} />
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
					<Card.Description>This transaction resulted in an exception</Card.Description>
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

		<!-- Segments Section -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Segments</Card.Title>
				<Card.Description>
					{#if response.hasSegments}
						Timing breakdown of operations within this transaction
					{:else}
						No segments recorded for this transaction
					{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				{#if response.hasSegments}
					<SegmentWaterfall
						segments={response.segments}
						transactionDuration={response.transaction.duration}
						transactionStartTime={response.transaction.recordedAt}
					/>
				{:else}
					<SegmentEmptyState framework={projectsState.currentProject?.framework ?? 'gin'} />
				{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</div>
