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
	import { Plus, Pencil, Trash2, Zap, ZapOff } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import IntegrationDialog from './integration-dialog.svelte';
	import type { Integration, ProviderInfo } from '$lib/types/agent';

	interface Props {
		organizationId: number;
	}

	let { organizationId }: Props = $props();

	let integrations = $state<Integration[]>([]);
	let providers = $state<ProviderInfo[]>([]);
	let loading = $state(true);
	let dialogOpen = $state(false);
	let editing = $state<Integration | null>(null);
	let deleting = $state<Integration | null>(null);
	let deleteOpen = $state(false);
	let deleteLoading = $state(false);
	let deleteError = $state('');

	async function load() {
		loading = true;
		try {
			const [list, known] = await Promise.all([
				api.get(`/organizations/${organizationId}/integrations`),
				api.get('/integrations/providers')
			]);
			integrations = list.integrations ?? [];
			providers = (known.providers ?? []).filter((p: ProviderInfo) => p.fields.length > 0);
		} catch (e) {
			toast.error(getErrorMessage(e, 'Failed to load integrations'));
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void organizationId;
		load();
	});

	async function toggle(integration: Integration) {
		try {
			await api.put(`/organizations/${organizationId}/integrations/${integration.id}`, {
				enabled: !integration.enabled
			});
			await load();
		} catch (e) {
			toast.error(getErrorMessage(e, 'Failed to update the integration'));
		}
	}

	async function remove() {
		if (!deleting) return;
		deleteLoading = true;
		deleteError = '';
		try {
			await api.delete(`/organizations/${organizationId}/integrations/${deleting.id}`);
			deleteOpen = false;
			toast.success('Successfully deleted the Integration', { position: 'top-center' });
			await load();
		} catch (e) {
			deleteError = getErrorMessage(e, 'Failed to delete the integration');
		} finally {
			deleteLoading = false;
		}
	}
</script>

<Card data-testid="integrations-card">
	<CardHeader class="flex flex-row items-start justify-between gap-4">
		<div>
			<CardTitle>Integrations</CardTitle>
			<CardDescription>
				Providers the fix agent works with: the GitHub credential that pushes its pull requests,
				chat tools that mirror its thread. Credentials are stored encrypted and never shown again.
			</CardDescription>
		</div>
		<Button
			variant="success"
			onclick={() => {
				editing = null;
				dialogOpen = true;
			}}
			disabled={providers.length === 0}
		>
			<Plus class="mr-2 h-4 w-4" />
			New Integration
		</Button>
	</CardHeader>
	<CardContent>
		{#if loading}
			<div class="flex justify-center py-8"><LoadingCircle size="lg" /></div>
		{:else}
			<TableContainer empty={integrations.length === 0}>
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Name</Table.Head>
							<Table.Head>Provider</Table.Head>
							<Table.Head>Kinds</Table.Head>
							<Table.Head>Enabled</Table.Head>
							<Table.Head class="text-right">Actions</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#if integrations.length === 0}
							<TableEmptyState colspan={5} message="No integrations yet." />
						{:else}
							{#each integrations as integration (integration.id)}
								<Table.Row>
									<Table.Cell class="font-medium">{integration.name}</Table.Cell>
									<Table.Cell>{integration.provider}</Table.Cell>
									<Table.Cell class="text-muted-foreground"
										>{integration.kinds.join(', ')}</Table.Cell
									>
									<Table.Cell>
										<StatusPill
											tone={integration.enabled ? 'success' : 'neutral'}
											label={integration.enabled ? 'On' : 'Off'}
										/>
									</Table.Cell>
									<Table.Cell>
										<div class="flex justify-end gap-1">
											<Button
												variant="ghost"
												size="icon"
												onclick={() => toggle(integration)}
												title={integration.enabled ? 'Disable' : 'Enable'}
											>
												{#if integration.enabled}<ZapOff class="h-4 w-4" />{:else}<Zap
														class="h-4 w-4"
													/>{/if}
											</Button>
											<Button
												variant="ghost"
												size="icon"
												title="Edit"
												onclick={() => {
													editing = integration;
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
													deleting = integration;
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

<IntegrationDialog
	bind:open={dialogOpen}
	{organizationId}
	{providers}
	integration={editing}
	onSaved={load}
/>

<ConfirmDeleteDialog
	bind:open={deleteOpen}
	entity="Integration"
	loading={deleteLoading}
	error={deleteError}
	onConfirm={remove}
/>
