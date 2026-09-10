<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Select from '$lib/components/ui/select';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { Check, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { resolveHref } from '$lib/utils/links';
	import { getErrorMessage } from '$lib/utils/errors';
	import type { CodeHostOption, Repository } from '$lib/types/agent';

	let repository = $state<Repository | null>(null);
	let codeHosts = $state<CodeHostOption[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let removing = $state(false);
	let error = $state('');
	let removeOpen = $state(false);

	let integrationId = $state('');
	let owner = $state('');
	let name = $state('');
	let defaultBranch = $state('main');
	let image = $state('');
	let setupCommand = $state('');
	let testCommand = $state('');

	const canWrite = $derived(projectsState.canWriteCurrentProject);

	async function load() {
		loading = true;
		try {
			const res = await api.get('/repositories', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			repository = res.repository ?? null;
			codeHosts = res.codeHosts ?? [];
			integrationId = repository?.integrationId ? String(repository.integrationId) : '';
			owner = repository?.owner ?? '';
			name = repository?.name ?? '';
			defaultBranch = repository?.defaultBranch || 'main';
			image = repository?.image ?? '';
			setupCommand = repository?.setupCommand ?? '';
			testCommand = repository?.testCommand ?? '';
		} catch (e) {
			error = getErrorMessage(e, 'Failed to load the repository');
		} finally {
			loading = false;
		}
	}

	async function save() {
		saving = true;
		error = '';
		try {
			const res = await api.put(
				'/repositories',
				{
					integrationId: integrationId ? Number(integrationId) : null,
					owner,
					name,
					defaultBranch,
					image,
					setupCommand,
					testCommand
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			repository = res.repository;
			toast.success('Successfully updated the Repository', { position: 'top-center' });
		} catch (e) {
			error = getErrorMessage(e, 'Failed to save the repository');
		} finally {
			saving = false;
		}
	}

	async function remove() {
		removing = true;
		try {
			await api.delete('/repositories', { projectId: projectsState.currentProjectId ?? undefined });
			removeOpen = false;
			await load();
			toast.success('Successfully removed the Repository', { position: 'top-center' });
		} catch (e) {
			error = getErrorMessage(e, 'Failed to remove the repository');
		} finally {
			removing = false;
		}
	}

	onMount(() => {
		load();
	});

	const selectedHost = $derived(codeHosts.find((h) => String(h.id) === integrationId) ?? null);
	const hostStatus = $derived(selectedHost?.status ?? '');
</script>

<Card>
	<CardHeader>
		<CardTitle>Repository</CardTitle>
		<CardDescription>
			Where this project's code lives. The agent clones it at the default branch and opens its pull
			requests there with the code host integration's credential.
		</CardDescription>
	</CardHeader>
	<CardContent>
		{#if loading}
			<div class="flex justify-center py-8"><LoadingCircle size="lg" /></div>
		{:else}
			<form
				class="max-w-xl space-y-4"
				data-testid="repository-form"
				onsubmit={(e) => {
					e.preventDefault();
					save();
				}}
			>
				<ErrorAlert {error} />
				<div class="space-y-2">
					<Label>Code host integration</Label>
					{#if codeHosts.length === 0}
						<p class="text-sm text-muted-foreground">
							No GitHub integration yet. An organization admin adds one under
							<a
								{...{ href: resolveHref('/settings') }}
								class="text-blue-600 hover:underline dark:text-blue-400">Settings</a
							>.
						</p>
					{:else}
						<Select.Root type="single" bind:value={integrationId} disabled={!canWrite}>
							<Select.Trigger class="w-full">
								{selectedHost
									? `${selectedHost.provider} · ${selectedHost.name}`
									: 'Select integration'}
							</Select.Trigger>
							<Select.Content>
								{#each codeHosts as host (host.id)}
									<Select.Item value={String(host.id)}>
										{host.provider} · {host.name}{host.status ? ` (${host.status})` : ''}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						{#if hostStatus}
							<p class="text-xs text-muted-foreground" data-testid="code-host-status">
								{hostStatus}
							</p>
						{/if}
					{/if}
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label for="repo-owner">Owner</Label>
						<Input
							id="repo-owner"
							bind:value={owner}
							placeholder="acme"
							disabled={!canWrite}
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="repo-name">Repository</Label>
						<Input
							id="repo-name"
							bind:value={name}
							placeholder="app"
							disabled={!canWrite}
							required
						/>
					</div>
				</div>
				<div class="space-y-2">
					<Label for="repo-branch">Default branch</Label>
					<Input
						id="repo-branch"
						bind:value={defaultBranch}
						placeholder="main"
						disabled={!canWrite}
					/>
				</div>
				<div class="space-y-2">
					<Label for="repo-image">Sandbox image (optional)</Label>
					<Input
						id="repo-image"
						bind:value={image}
						placeholder="ghcr.io/acme/toolchain:latest"
						disabled={!canWrite}
					/>
				</div>
				<div class="space-y-2">
					<Label for="repo-setup">Setup command (optional)</Label>
					<Input
						id="repo-setup"
						bind:value={setupCommand}
						placeholder="npm ci"
						disabled={!canWrite}
					/>
				</div>
				<div class="space-y-2">
					<Label for="repo-test">Test command (optional)</Label>
					<Input
						id="repo-test"
						bind:value={testCommand}
						placeholder="go test ./..."
						disabled={!canWrite}
					/>
					<p class="text-xs text-muted-foreground">
						Runs inside the sandbox with the network off after the agent's change; a failure goes
						back to the agent once.
					</p>
				</div>
				{#if canWrite}
					<div class="flex items-center gap-2">
						<Button type="submit" disabled={saving}>
							<Check class="mr-2 h-4 w-4" />
							{saving ? 'Saving...' : repository ? 'Update Repository' : 'Connect Repository'}
						</Button>
						{#if repository}
							<Button type="button" variant="outline" onclick={() => (removeOpen = true)}>
								<Trash2 class="mr-2 h-4 w-4" />
								Remove
							</Button>
						{/if}
					</div>
				{/if}
			</form>
		{/if}
	</CardContent>
</Card>

<ConfirmDeleteDialog
	bind:open={removeOpen}
	entity="Repository"
	verb="Remove"
	loading={removing}
	onConfirm={remove}
/>
