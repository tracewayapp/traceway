<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Avatar from '$lib/components/ui/avatar';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Plus, Pencil, Trash2 } from '@lucide/svelte';
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

	function openNewTeam() {
		editingTeam = null;
		teamDialogOpen = true;
	}

	function openEditTeam(team: Team) {
		editingTeam = team;
		teamDialogOpen = true;
	}

	async function handleDeleteTeam() {
		if (!teamToDelete) return;
		try {
			await oncallState.deleteTeam(organizationId, teamToDelete.id);
			toast.success('Successfully deleted the Team', { position: 'top-center' });
			oncallState.loadSchedules(organizationId);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to delete team', {
				position: 'top-center'
			});
		} finally {
			teamToDelete = null;
		}
	}
</script>

<div class="space-y-4">
	{#if canManage}
		<div class="flex justify-end">
			<Button size="sm" variant="success" onclick={openNewTeam}>
				<Plus class="mr-1 h-4 w-4" /> New Team
			</Button>
		</div>
	{/if}

	{#if oncallState.teamsLoading}
		<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if oncallState.teamsError}
		<div
			class="flex flex-col items-center justify-center gap-3 rounded-md bg-muted py-20 text-center text-muted-foreground"
		>
			<p class="text-sm text-destructive">{oncallState.teamsError}</p>
			<Button variant="outline" size="sm" onclick={() => oncallState.loadTeams(organizationId)}>
				Retry
			</Button>
		</div>
{:else if teams.length === 0}
		<div
			class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
		>
			<p class="mb-4">No teams yet. Create a team to get started.</p>
			{#if canManage}
				<Button variant="success" onclick={openNewTeam}>
					<Plus class="mr-1 h-4 w-4" />
					Create your first Team
				</Button>
			{/if}
		</div>
	{:else}
		<div class="overflow-hidden rounded-md border">
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
											onclick={() => (teamToDelete = team)}
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
		</div>
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

<AlertDialog.Root
	open={teamToDelete !== null}
	onOpenChange={(open) => {
		if (!open) teamToDelete = null;
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Team</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{teamToDelete?.name}"? Its schedules will be deleted as
				well. This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
			<AlertDialog.Action variant="destructive" onclick={handleDeleteTeam}>
				<Trash2 class="mr-2 h-4 w-4" />
				Delete Team
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
