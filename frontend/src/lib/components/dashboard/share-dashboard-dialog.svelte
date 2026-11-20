<script lang="ts">
	import { onMount } from 'svelte';
	import { Check, Copy, RefreshCw } from '@lucide/svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { REPLAY_BLOCK_CLASS } from '$lib/utils/self-monitoring';
	import { api } from '$lib/api';

	let {
		dashboardId,
		projectId,
		view,
		onClose
	}: {
		dashboardId: string;
		projectId: string;
		view: { from: string; to: string; tagFilters: Record<string, string> };
		onClose: () => void;
	} = $props();

	let url = $state('');
	let error = $state('');
	let loading = $state(true);
	let expired = $state(false);
	let copied = $state(false);
	let expiryTimer: ReturnType<typeof setTimeout>;
	let copiedTimer: ReturnType<typeof setTimeout>;
	const endpoint = $derived(`/dashboards/${dashboardId}/embed-token`);

	async function generate() {
		clearTimeout(expiryTimer);
		loading = true;
		error = '';
		expired = false;
		copied = false;
		try {
			const result: { embedPath: string; expiresInSeconds: number } = await api.post(
				endpoint,
				view,
				{ projectId }
			);
			url = new URL(result.embedPath, window.location.origin).href;
			expiryTimer = setTimeout(() => (expired = true), result.expiresInSeconds * 1000);
		} catch (e) {
			url = '';
			error = getErrorMessage(e) || 'Could not generate a dashboard link.';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		generate();
		return () => {
			clearTimeout(expiryTimer);
			clearTimeout(copiedTimer);
		};
	});

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(url);
			copied = true;
			clearTimeout(copiedTimer);
			copiedTimer = setTimeout(() => (copied = false), 2000);
		} catch {
			error = 'Select the link and copy it manually.';
		}
	}
</script>

<AlertDialog.Root
	open
	onOpenChange={(open) => {
		if (!open) onClose();
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Share Dashboard</AlertDialog.Title>
			<AlertDialog.Description>
				Anyone with this link can view the dashboard with the selected time range and filters,
				without signing in. The link works for five minutes.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert {error} />
		{#if loading}
			<div class="flex items-center gap-2 text-sm text-muted-foreground" role="status">
				<LoadingCircle size="sm" />
				Generating link…
			</div>
		{:else if url}
			<div class={REPLAY_BLOCK_CLASS}>
				<FormField
					label="Link"
					for="dashboard-embed-url"
					hint={expired ? 'This link has expired. Generate a new one to share.' : undefined}
				>
					<Input
						id="dashboard-embed-url"
						readonly
						value={url}
						disabled={expired}
						onclick={(e) => e.currentTarget.select()}
					/>
				</FormField>
			</div>
		{/if}
		<div class="space-y-2 border-t pt-4 text-sm text-muted-foreground">
			<p>
				To embed this dashboard in your own application, have your backend request a fresh link for
				each authorized viewer.
				<a
					href="https://docs.tracewayapp.com/learn/dashboards#share-and-embed-a-dashboard"
					target="_blank"
					rel="noopener noreferrer"
					class="underline hover:text-foreground">Embedding guide</a
				>
			</p>
			<code class="block rounded bg-muted px-2 py-1.5 font-mono text-xs break-all select-all"
				>POST /api{endpoint}?projectId={projectId}</code
			>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={onClose}>Close</Button>
			{#if loading}
				<Button disabled><Copy />Copy Link</Button>
			{:else if expired || !url}
				<Button onclick={generate}><RefreshCw />{url ? 'New Link' : 'Try Again'}</Button>
			{:else}
				<Button onclick={copyLink}>
					{#if copied}<Check />Copied{:else}<Copy />Copy Link{/if}
				</Button>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
