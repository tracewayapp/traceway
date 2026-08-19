<script lang="ts">
	import { Check, Bot } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { getFrameworkLabel } from '$lib/state/projects.svelte';
	import type { SetupDraft } from '$lib/utils/setup-plan';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';

	let { draft, onDecided }: { draft: SetupDraft; onDecided: () => void } = $props();

	let loading = $state(false);
	let error = $state('');
	let rejecting = $state(false);
	let rejectReason = $state('');

	async function approve() {
		loading = true;
		error = '';
		try {
			await api.post(`/setup/drafts/${draft.id}/approve`, {});
			toast.success('Successfully created the Projects', { position: 'top-center' });
			onDecided();
		} catch (e: unknown) {
			const err = e as Error & { status?: number };
			if (err.status === 409) {
				onDecided();
			} else {
				error = err.message || 'Failed to approve the proposal';
			}
		} finally {
			loading = false;
		}
	}

	async function reject() {
		loading = true;
		error = '';
		try {
			await api.post(`/setup/drafts/${draft.id}/reject`, { reason: rejectReason.trim() });
			onDecided();
		} catch (e: unknown) {
			const err = e as Error & { status?: number };
			if (err.status === 409) {
				onDecided();
			} else {
				error = err.message || 'Failed to reject the proposal';
			}
		} finally {
			loading = false;
		}
	}
</script>

<div class="rounded-md border-2 border-primary/40 bg-card">
	<div class="border-b px-4 py-3">
		<div class="flex items-center gap-3">
			<Bot class="h-5 w-5 text-primary" />
			<h3 class="font-semibold">Your AI assistant proposed this setup</h3>
		</div>
		<p class="mt-1 ml-8 text-sm text-muted-foreground">
			Requested by {draft.requestedByEmail}. Nothing is created until you approve it.
		</p>
	</div>
	<div class="space-y-3 p-4">
		{#if error}
			<ErrorAlert {error} />
		{/if}

		<ul class="space-y-2">
			{#each draft.payload.projects as project (project.name)}
				<li class="rounded-md border px-3 py-2.5">
					<div class="flex items-center gap-3">
						<FrameworkIcon framework={project.framework} class="size-5 shrink-0" />
						<div class="min-w-0 flex-1">
							<div class="truncate text-sm font-medium">{project.name}</div>
							<div class="text-xs text-muted-foreground">
								{getFrameworkLabel(project.framework as never) || project.framework}
							</div>
						</div>
					</div>
					{#if project.envFile || project.deployment}
						<div class="mt-2 ml-8 space-y-1 text-xs text-muted-foreground">
							{#if project.envFile}
								<div>
									Token written to <code class="rounded bg-muted px-1 py-0.5"
										>{project.envFile}</code
									>
									as
									<code class="rounded bg-muted px-1 py-0.5">{project.envVar}</code>
								</div>
							{/if}
							{#if project.deployment}
								<div>
									Deployment: {project.deployment.platform} (instructions shown here after
									approval)
								</div>
							{/if}
						</div>
					{/if}
				</li>
			{/each}
		</ul>

		{#if rejecting}
			<div class="space-y-2">
				<Input
					type="text"
					placeholder="Why? Your agent sees this and revises the plan (optional)"
					bind:value={rejectReason}
					disabled={loading}
				/>
				<div class="flex justify-end gap-2">
					<Button variant="outline" onclick={() => (rejecting = false)} disabled={loading}>
						Back
					</Button>
					<Button variant="destructive" onclick={reject} disabled={loading}>
						Reject Proposal
					</Button>
				</div>
			</div>
		{:else}
			<div class="flex justify-end gap-2">
				<Button variant="outline" onclick={() => (rejecting = true)} disabled={loading}>
					Reject
				</Button>
				<Button onclick={approve} disabled={loading}>
					{#if loading}
						Approving...
					{:else}
						<Check class="mr-2 h-4 w-4" />
						Approve Setup
					{/if}
				</Button>
			</div>
		{/if}
	</div>
</div>
