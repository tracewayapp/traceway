<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';
	import { ArrowLeft, ArrowRight, TriangleAlert } from 'lucide-svelte';
	import SegmentWaterfall from '$lib/components/segments/segment-waterfall.svelte';
	import SegmentEmptyState from '$lib/components/segments/segment-empty-state.svelte';
	import type { TransactionDetailResponse } from '$lib/types/segments';

	let { data } = $props();

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

	function formatDuration(nanoseconds: number): string {
		const ms = nanoseconds / 1_000_000;
		if (ms < 1) {
			return `${(nanoseconds / 1000).toFixed(2)}us`;
		} else if (ms < 1000) {
			return `${ms.toFixed(2)}ms`;
		} else {
			return `${(ms / 1000).toFixed(2)}s`;
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

	function getStatusColor(statusCode: number): string {
		if (statusCode >= 200 && statusCode < 300) return 'text-green-500';
		if (statusCode >= 300 && statusCode < 400) return 'text-blue-500';
		if (statusCode >= 400 && statusCode < 500) return 'text-yellow-500';
		return 'text-red-500';
	}

	function getBackUrl(): string {
		const params = new URLSearchParams();
		if (data.preset) {
			params.set('preset', data.preset);
		} else if (data.from && data.to) {
			params.set('from', data.from);
			params.set('to', data.to);
		}
		const queryString = params.toString();
		return `/transactions/${encodeURIComponent(data.endpoint)}${queryString ? '?' + queryString : ''}`;
	}

	function goBack() {
		goto(getBackUrl());
	}

	const backHref = $derived(getBackUrl());

	onMount(() => {
		loadData();
	});
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center gap-4">
		<Button variant="ghost" size="sm" onclick={goBack} class="h-8 w-8 p-0">
			<ArrowLeft class="h-4 w-4" />
		</Button>
		<div>
			<h2 class="font-mono text-2xl font-bold tracking-tight">
				{decodeURIComponent(data.endpoint)}
			</h2>
			<p class="text-muted-foreground text-sm">Transaction ID: {data.transactionId}</p>
		</div>
	</div>

	{#if loading}
		<Card.Root>
			<Card.Header>
				<Skeleton class="h-6 w-48" />
			</Card.Header>
			<Card.Content>
				<Skeleton class="mb-2 h-4 w-full" />
				<Skeleton class="mb-2 h-4 w-3/4" />
				<Skeleton class="h-32 w-full" />
			</Card.Content>
		</Card.Root>
	{:else if notFound}
		<ErrorDisplay
			status={404}
			title="Transaction Not Found"
			description="The transaction you're looking for doesn't exist or may have expired."
			backHref={backHref}
			backLabel="Back to Endpoint"
			onRetry={loadData}
			identifier={data.transactionId}
		/>
	{:else if error}
		<ErrorDisplay
			status={400}
			title="Failed to Load Transaction"
			description={error}
			backHref={backHref}
			backLabel="Back to Endpoint"
			onRetry={loadData}
		/>
	{:else if response}
		<!-- Transaction Summary Card -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Transaction Details</Card.Title>
				<Card.Description>
					Recorded: {new Date(response.transaction.recordedAt).toLocaleString()}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Duration</p>
						<p class="font-mono text-lg">{formatDuration(response.transaction.duration)}</p>
					</div>
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Status</p>
						<p class="font-mono text-lg {getStatusColor(response.transaction.statusCode)}">
							{response.transaction.statusCode}
						</p>
					</div>
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Body Size</p>
						<p class="font-mono">{formatBytes(response.transaction.bodySize)}</p>
					</div>
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Client IP</p>
						<p class="font-mono">{response.transaction.clientIP || '-'}</p>
					</div>
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Server</p>
						<p class="font-mono">{response.transaction.serverName || '-'}</p>
					</div>
					<div class="space-y-1">
						<p class="text-muted-foreground text-sm">Version</p>
						<p class="font-mono">{response.transaction.appVersion || '-'}</p>
					</div>
				</div>
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
					<div class="bg-muted rounded-md p-3 overflow-x-auto max-h-32 mb-4">
						<pre class="text-sm font-mono whitespace-pre-wrap">{response.exception.stackTrace.split('\n').slice(0, 4).join('\n')}{response.exception.stackTrace.split('\n').length > 4 ? '\n...' : ''}</pre>
					</div>
					<Button variant="outline" size="sm" onclick={() => goto(`/issues/${response!.exception!.exceptionHash}`)}>
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

		<!-- Context Card (if scope exists) -->
		{#if response.transaction.scope && Object.keys(response.transaction.scope).length > 0}
			<Card.Root>
				<Card.Header>
					<Card.Title>Context</Card.Title>
					<Card.Description>Additional context captured with this transaction</Card.Description>
				</Card.Header>
				<Card.Content>
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
						{#each Object.entries(response.transaction.scope).sort((a, b) => a[0].localeCompare(b[0])) as [key, value]}
							<div class="bg-muted flex flex-col gap-1 rounded-md p-3">
								<span class="text-muted-foreground text-xs font-medium">{key}</span>
								<span class="break-all font-mono text-sm">{value}</span>
							</div>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>
		{/if}
	{/if}
</div>
