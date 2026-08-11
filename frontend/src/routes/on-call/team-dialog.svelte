<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus, Check, ChevronUp, ChevronDown, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { oncallState, type Team, type OrgMember } from '$lib/state/oncall.svelte';
	import { projectsState } from '$lib/state/projects.svelte';

	interface Props {
		open: boolean;
		organizationId: number;
		team: Team | null;
		onSaved: () => void;
	}

	let { open = $bindable(), organizationId, team, onSaved }: Props = $props();

	let name = $state('');
	let description = $state('');
	let selectedUserIds = $state<number[]>([]);
	let selectedProjectIds = $state<string[]>([]);
	let members = $state<OrgMember[]>([]);
	let membersLoading = $state(false);
	let loading = $state(false);
	let error = $state('');

	const isEditing = $derived(team !== null);

	const orgProjects = $derived(
		projectsState.projects.filter((p) => p.organizationId === organizationId)
	);

	const selectedMembers = $derived(
		selectedUserIds
			.map((id) => members.find((m) => m.id === id))
			.filter((m): m is OrgMember => m !== undefined)
	);

	async function loadMembers() {
		membersLoading = true;
		try {
			members = await oncallState.getMembers(organizationId);
		} catch {
			members = [];
		} finally {
			membersLoading = false;
		}
	}

	function resetForm() {
		name = '';
		description = '';
		selectedUserIds = [];
		selectedProjectIds = [];
		error = '';
	}

	function populateFromTeam(t: Team) {
		name = t.name;
		description = t.description;
		selectedUserIds = [...t.members]
			.sort((a, b) => a.position - b.position)
			.map((m) => m.userId);
		selectedProjectIds = t.projects.map((p) => p.projectId);
		error = '';
	}

	function toggleMember(userId: number, checked: boolean) {
		if (checked) {
			if (!selectedUserIds.includes(userId)) {
				selectedUserIds = [...selectedUserIds, userId];
			}
		} else {
			selectedUserIds = selectedUserIds.filter((id) => id !== userId);
		}
	}

	function moveMember(index: number, direction: -1 | 1) {
		const target = index + direction;
		if (target < 0 || target >= selectedUserIds.length) return;
		const next = [...selectedUserIds];
		[next[index], next[target]] = [next[target], next[index]];
		selectedUserIds = next;
	}

	function toggleProject(projectId: string, checked: boolean) {
		if (checked) {
			if (!selectedProjectIds.includes(projectId)) {
				selectedProjectIds = [...selectedProjectIds, projectId];
			}
		} else {
			selectedProjectIds = selectedProjectIds.filter((id) => id !== projectId);
		}
	}

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			if (isEditing && team) {
				await oncallState.updateTeam(organizationId, team.id, {
					name,
					description,
					memberUserIds: selectedUserIds,
					projectIds: selectedProjectIds
				});
				toast.success('Successfully updated the Team', { position: 'top-center' });
			} else {
				await oncallState.createTeam(organizationId, {
					name,
					description,
					memberUserIds: selectedUserIds,
					projectIds: selectedProjectIds
				});
				toast.success('Successfully created the Team', { position: 'top-center' });
			}
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save team';
		} finally {
			loading = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		open = isOpen;
	}

	$effect(() => {
		if (open) {
			if (team) {
				populateFromTeam(team);
			} else {
				resetForm();
			}
			loadMembers();
		}
	});
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
	<AlertDialog.Content class="max-h-[90vh] max-w-lg overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Team' : 'New Team'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the team, its members and owned projects'
					: 'Create a team with members and owned projects'}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<ErrorAlert {error} />

			<div class="space-y-2">
				<Label for="team-name">Name</Label>
				<Input id="team-name" bind:value={name} placeholder="e.g. Platform" />
			</div>

			<div class="space-y-2">
				<Label for="team-description">Description</Label>
				<Input id="team-description" bind:value={description} placeholder="What this team owns" />
			</div>

			<div class="space-y-2">
				<Label>Members</Label>
				{#if membersLoading}
					<p class="text-sm text-muted-foreground">Loading members...</p>
				{:else if members.length === 0}
					<p class="text-sm text-muted-foreground">No members found.</p>
				{:else}
					<div class="max-h-44 space-y-1 overflow-y-auto rounded-md border p-2">
						{#each members as member (member.id)}
							<label class="flex cursor-pointer items-center gap-2 rounded px-1 py-0.5 hover:bg-accent">
								<Checkbox
									checked={selectedUserIds.includes(member.id)}
									onCheckedChange={(checked) => toggleMember(member.id, checked === true)}
									aria-label="Select {member.name}"
								/>
								<span class="text-sm">{member.name}</span>
								<span class="text-xs text-muted-foreground">{member.email}</span>
							</label>
						{/each}
					</div>
				{/if}
			</div>

			{#if selectedMembers.length > 0}
				<div class="space-y-2">
					<Label>Member order</Label>
					<div class="space-y-1 rounded-md border p-2">
						{#each selectedMembers as member, index (member.id)}
							<div class="flex items-center gap-2">
								<span class="w-5 text-xs text-muted-foreground">{index + 1}.</span>
								<span class="flex-1 truncate text-sm">{member.name}</span>
								<Button
									variant="ghost"
									size="icon"
									type="button"
									class="h-6 w-6"
									disabled={index === 0}
									onclick={() => moveMember(index, -1)}
								>
									<ChevronUp class="h-3.5 w-3.5" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									type="button"
									class="h-6 w-6"
									disabled={index === selectedMembers.length - 1}
									onclick={() => moveMember(index, 1)}
								>
									<ChevronDown class="h-3.5 w-3.5" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									type="button"
									class="h-6 w-6"
									onclick={() => toggleMember(member.id, false)}
								>
									<X class="h-3.5 w-3.5" />
								</Button>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<div class="space-y-2">
				<Label>Owned projects</Label>
				{#if orgProjects.length === 0}
					<p class="text-sm text-muted-foreground">No projects in this organization.</p>
				{:else}
					<div class="max-h-36 space-y-1 overflow-y-auto rounded-md border p-2">
						{#each orgProjects as project (project.id)}
							<label class="flex cursor-pointer items-center gap-2 rounded px-1 py-0.5 hover:bg-accent">
								<Checkbox
									checked={selectedProjectIds.includes(project.id)}
									onCheckedChange={(checked) => toggleProject(project.id, checked === true)}
									aria-label="Select {project.name}"
								/>
								<span class="text-sm">{project.name}</span>
							</label>
						{/each}
					</div>
				{/if}
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Team'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Team'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
