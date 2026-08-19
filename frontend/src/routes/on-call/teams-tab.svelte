<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import * as Avatar from '$lib/components/ui/avatar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { Pencil, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { oncallState, type Team } from '$lib/state/oncall.svelte';
	import TeamDialog from './team-dialog.svelte';

	interface Props {
		organizationId: number;
		canManage: boolean;
	}

	let { organizationId, canManage }: Props = $props();

	let teamDialogOpen = $state(false);
	let editingTeam = $state<Team | null>(null);
	let teamToDelete = $state<Team | null>(null);
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);

	const teams = $derived(oncallState.teams);

	function initials(name: string, email: string): string {
		const source = name || email;
		return source
			.split(/[\s@._-]+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((p) => p[0]!.toUpperCase())
			.join('');
	}

	export function openNew() {
		editingTeam = null;
		teamDialogOpen = true;
	}

	function openEditTeam(team: Team) {
		editingTeam = team;
		teamDialogOpen = true;
	}

	async function handleDeleteTeam() {
		if (!teamToDelete) return;
		deleting = true;
		try {
			await oncallState.deleteTeam(organizationId, teamToDelete.id);
			toast.success('Successfully deleted the Team');
			oncallState.loadSchedules(organizationId);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete team');
		} finally {
			deleting = false;
			deleteDialogOpen = false;
			teamToDelete = null;
		}
	}
</script>

<div class="space-y-4">
	{#if oncallState.teamsLoading}
		<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
	{:else if oncallState.teamsError}
		<ErrorRetryBox
			message={oncallState.teamsError}
			onRetry={() => oncallState.loadTeams(organizationId)}
		/>
	{:else if teams.length === 0}
		<EmptyState
			message="No teams yet. Create a team to get started."
			actionLabel={canManage ? 'Create your first Team' : undefined}
			onAction={openNew}
		/>
	{:else}
		<TableContainer>
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Name</Table.Head>
						<Table.Head>Members</Table.Head>
						<Table.Head>Projects</Table.Head>
						<Table.Head>Schedules</Table.Head>
						{#if canManage}
							<Table.Head class="text-right">Actions</Table.Head>
						{/if}
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each teams as team (team.id)}
						<Table.Row>
							<Table.Cell>
								<div class="font-medium">{team.name}</div>
								{#if team.description}
									<div class="text-xs text-muted-foreground">{team.description}</div>
								{/if}
							</Table.Cell>
							<Table.Cell>
								<div class="flex items-center gap-2">
									<div class="flex -space-x-2">
										{#each team.members.slice(0, 5) as member (member.userId)}
											<Avatar.Root
												class="h-6 w-6 border-2 border-background"
												title="{member.name} ({member.email})"
											>
												<Avatar.Fallback class="text-[10px]">
													{initials(member.name, member.email)}
												</Avatar.Fallback>
											</Avatar.Root>
										{/each}
									</div>
									<span class="text-sm text-muted-foreground">{team.memberCount}</span>
								</div>
							</Table.Cell>
							<Table.Cell>
								<div class="flex flex-wrap gap-1">
									{#each team.projects as project (project.projectId)}
										<Badge variant="outline">{project.name}</Badge>
									{:else}
										<span class="text-sm text-muted-foreground">—</span>
									{/each}
								</div>
							</Table.Cell>
							<Table.Cell>{team.scheduleCount}</Table.Cell>
							{#if canManage}
								<Table.Cell class="text-right">
									<div class="flex justify-end gap-1">
										<Button
											variant="ghost"
											size="icon"
											title="Edit"
											onclick={() => openEditTeam(team)}
										>
											<Pencil class="h-4 w-4" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											title="Delete"
											onclick={() => {
												teamToDelete = team;
												deleteDialogOpen = true;
											}}
										>
											<Trash2 class="h-4 w-4" />
										</Button>
									</div>
								</Table.Cell>
							{/if}
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</TableContainer>
	{/if}
</div>

<TeamDialog
	bind:open={teamDialogOpen}
	{organizationId}
	team={editingTeam}
	onSaved={() => {
		teamDialogOpen = false;
		oncallState.loadTeams(organizationId);
	}}
/>

<ConfirmDeleteDialog
	bind:open={deleteDialogOpen}
	entity="Team"
	description={`Are you sure you want to delete "${teamToDelete?.name}"? Its schedules will be deleted as well. This action cannot be undone.`}
	loading={deleting}
	onConfirm={handleDeleteTeam}
/>
