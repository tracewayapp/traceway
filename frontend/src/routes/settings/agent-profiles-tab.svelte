<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Plus, Pencil, Trash2, Star } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import AgentProfileDialog from './agent-profile-dialog.svelte';
	import type { AgentProfile } from '$lib/types/agent';

	interface Props {
		organizationId: number;
	}

	let { organizationId }: Props = $props();

	let profiles = $state<AgentProfile[]>([]);
	let loading = $state(true);
	let dialogOpen = $state(false);
	let editing = $state<AgentProfile | null>(null);
	let deleting = $state<AgentProfile | null>(null);
	let deleteOpen = $state(false);
	let deleteLoading = $state(false);
	let deleteError = $state('');

	async function load() {
		loading = true;
		try {
			const res = await api.get(`/organizations/${organizationId}/agent-profiles`);
			profiles = res.profiles ?? [];
		} catch (e) {
			toast.error(getErrorMessage(e, 'Failed to load agent profiles'));
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void organizationId;
		load();
	});

	async function makeDefault(profile: AgentProfile) {
		try {
			await api.post(`/organizations/${organizationId}/agent-profiles/${profile.id}/default`, {});
			await load();
		} catch (e) {
			toast.error(getErrorMessage(e, 'Failed to set the default profile'));
		}
	}

	async function remove() {
		if (!deleting) return;
		deleteLoading = true;
		deleteError = '';
		try {
			await api.delete(`/organizations/${organizationId}/agent-profiles/${deleting.id}`);
			deleteOpen = false;
			toast.success('Successfully deleted the Profile', { position: 'top-center' });
			await load();
		} catch (e) {
			deleteError = getErrorMessage(e, 'Failed to delete the profile');
		} finally {
			deleteLoading = false;
		}
	}
</script>

<Card data-testid="agent-profiles-card">
	<CardHeader class="flex flex-row items-start justify-between gap-4">
		<div>
			<CardTitle>Agent profiles</CardTitle>
			<CardDescription>
				The coding agent, model, key and limits an attempt runs with. The default profile is what
				Fix it uses unless another is picked.
			</CardDescription>
		</div>
		<Button
			variant="success"
			onclick={() => {
				editing = null;
				dialogOpen = true;
			}}
		>
			<Plus class="mr-2 h-4 w-4" />
			New Profile
		</Button>
	</CardHeader>
	<CardContent>
		{#if loading}
			<div class="flex justify-center py-8"><LoadingCircle size="lg" /></div>
		{:else}
			<TableContainer empty={profiles.length === 0}>
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Name</Table.Head>
							<Table.Head>Agent</Table.Head>
							<Table.Head>Model</Table.Head>
							<Table.Head>Key</Table.Head>
							<Table.Head>Limits</Table.Head>
							<Table.Head class="text-right">Actions</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#if profiles.length === 0}
							<TableEmptyState colspan={6} message="No agent profiles yet." />
						{:else}
							{#each profiles as profile (profile.id)}
								<Table.Row>
									<Table.Cell class="font-medium">
										{profile.name}
										{#if profile.isDefault}
											<StatusPill tone="info" label="default" class="ml-2" />
										{/if}
									</Table.Cell>
									<Table.Cell>{profile.agent}</Table.Cell>
									<Table.Cell class="text-muted-foreground">{profile.model || '-'}</Table.Cell>
									<Table.Cell>
										<StatusPill
											tone={profile.hasCredential ? 'success' : 'warning'}
											label={profile.hasCredential ? 'set' : 'missing'}
										/>
									</Table.Cell>
									<Table.Cell class="text-xs text-muted-foreground">
										{profile.maxTurns || '∞'} turns · {profile.timeoutMinutes || '∞'} min · ${profile.budgetUsd}
									</Table.Cell>
									<Table.Cell>
										<div class="flex justify-end gap-1">
											{#if !profile.isDefault}
												<Button
													variant="ghost"
													size="icon"
													title="Make default"
													onclick={() => makeDefault(profile)}
												>
													<Star class="h-4 w-4" />
												</Button>
											{/if}
											<Button
												variant="ghost"
												size="icon"
												title="Edit"
												onclick={() => {
													editing = profile;
													dialogOpen = true;
												}}
											>
												<Pencil class="h-4 w-4" />
											</Button>
											<Button
												variant="ghost"
												size="icon"
												title="Delete"
												onclick={() => {
													deleting = profile;
													deleteOpen = true;
												}}
											>
												<Trash2 class="h-4 w-4" />
											</Button>
										</div>
									</Table.Cell>
								</Table.Row>
							{/each}
						{/if}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		{/if}
	</CardContent>
</Card>

<AgentProfileDialog bind:open={dialogOpen} {organizationId} profile={editing} onSaved={load} />

<ConfirmDeleteDialog
	bind:open={deleteOpen}
	entity="Profile"
	loading={deleteLoading}
	error={deleteError}
	onConfirm={remove}
/>
