<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { Pencil, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import {
		oncallState,
		type EscalationPolicy,
		type PolicyTargetType
	} from '$lib/state/oncall.svelte';
	import EscalationChain from '$lib/components/traceway/escalation-chain.svelte';
	import PolicyDialog from './policy-dialog.svelte';

	interface Props {
		organizationId: number;
		canManage: boolean;
	}

	let { organizationId, canManage }: Props = $props();

	interface TargetOption {
		id: number;
		label: string;
	}

	let memberOptions = $state<TargetOption[]>([]);
	let channelOptions = $state<TargetOption[]>([]);

	let policyDialogOpen = $state(false);
	let editingPolicy = $state<EscalationPolicy | null>(null);

	let deletingPolicy = $state<EscalationPolicy | null>(null);
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let deleteError = $state('');

	const scheduleOptions = $derived<TargetOption[]>(
		oncallState.schedules.map((s) => ({ id: s.id, label: s.name }))
	);
	const teamOptions = $derived<TargetOption[]>(
		oncallState.teams.map((t) => ({ id: t.id, label: t.name }))
	);

	const targetOptions = $derived<Record<PolicyTargetType, TargetOption[]>>({
		schedule: scheduleOptions,
		user: memberOptions,
		team: teamOptions,
		channel: channelOptions
	});

	const labels = $derived.by(() => {
		const result: Record<string, string> = {};
		for (const [type, options] of Object.entries(targetOptions)) {
			for (const option of options) {
				result[`${type}:${option.id}`] = option.label;
			}
		}
		return result;
	});

	$effect(() => {
		oncallState.loadPolicies(organizationId);
	});

	$effect(() => {
		if (canManage) {
			oncallState
				.getMembers(organizationId)
				.then((members) => {
					memberOptions = (members || []).map((m) => ({ id: m.id, label: m.name || m.email }));
				})
				.catch(() => {
					memberOptions = [];
				});
		} else {
			memberOptions = [];
		}
	});

	$effect(() => {
		const orgProjects = projectsState.projects.filter((p) => p.organizationId === organizationId);
		Promise.all(
			orgProjects.map((project) =>
				api
					.get('/notification-channels', { projectId: project.id })
					.then((res) =>
						((res.channels || []) as { id: number; name: string; channelType: string }[])
							.filter((ch) => ch.channelType !== 'escalation')
							.map((ch) => ({ id: ch.id, label: `${project.name}: ${ch.name}` }))
					)
					.catch(() => [] as TargetOption[])
			)
		).then((results) => {
			channelOptions = results.flat();
		});
	});

	export function openNew() {
		editingPolicy = null;
		policyDialogOpen = true;
	}

	function openEdit(policy: EscalationPolicy) {
		editingPolicy = policy;
		policyDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deletingPolicy) return;
		deleting = true;
		deleteError = '';
		try {
			await oncallState.deletePolicy(organizationId, deletingPolicy.id);
			toast.success('Successfully deleted the Policy');
			deleteDialogOpen = false;
			deletingPolicy = null;
		} catch (e: unknown) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete policy';
		} finally {
			deleting = false;
		}
	}
</script>

<div class="space-y-4">
	{#if oncallState.policiesLoading}
		<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
	{:else if oncallState.policiesError}
		<ErrorRetryBox
			message={oncallState.policiesError}
			onRetry={() => oncallState.loadPolicies(organizationId)}
		/>
	{:else if oncallState.policies.length === 0}
		<EmptyState
			message="No escalation policies yet. Define who gets paged, and in what order."
			actionLabel={canManage ? 'Create your first Policy' : undefined}
			onAction={openNew}
		/>
	{:else}
		<div class="space-y-3">
			{#each oncallState.policies as policy (policy.id)}
				<Card>
					<CardHeader class="pb-2">
						<div class="flex items-start justify-between gap-2">
							<div>
								<CardTitle class="text-base">{policy.name}</CardTitle>
								<CardDescription>
									{policy.definition?.steps?.length ?? 0} step{(policy.definition?.steps?.length ??
										0) === 1
										? ''
										: 's'}
								</CardDescription>
							</div>
							{#if canManage}
								<div class="flex gap-1">
									<Button variant="ghost" size="icon" title="Edit" onclick={() => openEdit(policy)}>
										<Pencil class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										title="Delete"
										onclick={() => {
											deleteError = '';
											deletingPolicy = policy;
											deleteDialogOpen = true;
										}}
									>
										<Trash2 class="h-4 w-4" />
									</Button>
								</div>
							{/if}
						</div>
					</CardHeader>
					<CardContent>
						<EscalationChain definition={policy.definition} {labels} compact />
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>

<PolicyDialog
	bind:open={policyDialogOpen}
	{organizationId}
	policy={editingPolicy}
	{targetOptions}
	{labels}
	onSaved={() => {
		policyDialogOpen = false;
	}}
/>

<ConfirmDeleteDialog
	bind:open={deleteDialogOpen}
	entity="Policy"
	description={`Are you sure you want to delete "${deletingPolicy?.name}"? This action cannot be undone.`}
	error={deleteError}
	loading={deleting}
	onConfirm={confirmDelete}
/>
