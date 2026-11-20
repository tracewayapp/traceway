<script lang="ts">
	import { onMount } from 'svelte';
	import { Copy } from '@lucide/svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
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
	let copied = $state(false);
	const endpoint = $derived(`/dashboards/${dashboardId}/embed-token`);

	onMount(async () => {
		try {
			const result = await api.post(endpoint, view, { projectId });
			url = new URL(result.embedPath, window.location.origin).href;
		} catch (e) {
			error = getErrorMessage(e) || 'Could not generate a dashboard link.';
		}
	});

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(url);
			copied = true;
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
			<AlertDialog.Description
				>Anyone with this link can view the selected time range and filters for five minutes.</AlertDialog.Description
			>
		</AlertDialog.Header>
		<ErrorAlert {error} />
		{#if url}
			<div class={REPLAY_BLOCK_CLASS}>
				<FormField label="Embed link" for="dashboard-embed-url">
					<Input
						id="dashboard-embed-url"
						readonly
						value={url}
						onclick={(e) => e.currentTarget.select()}
					/>
				</FormField>
			</div>
		{:else if !error}
			<p role="status">Generating link…</p>
		{/if}
		<p class="text-sm text-muted-foreground">
			For an embedded dashboard, have your backend request a fresh link for each authorized viewer.
		</p>
		<code class="text-xs break-all select-all">POST /api{endpoint}?projectId={projectId}</code>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={onClose}>Close</Button>
			{#if url}<Button onclick={copyLink}><Copy />{copied ? 'Copied' : 'Copy link'}</Button>{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
