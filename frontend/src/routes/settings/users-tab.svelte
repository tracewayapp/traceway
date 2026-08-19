<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { Badge } from '$lib/components/ui/badge';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { Plus, Trash2, ChevronRight, ChevronDown } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		organizationState,
		type OrganizationMember,
		type Invitation,
		type MemberProjectRole
	} from '$lib/state/organization.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { type Framework } from '$lib/state/projects.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import InviteUserDialog from './invite-user-dialog.svelte';

	interface Props {
		organizationId: number;
	}

	let { organizationId }: Props = $props();

	let showInviteDialog = $state(false);
	let memberToRemove = $state<OrganizationMember | null>(null);
	let removeDialogOpen = $state(false);
	let removeLoading = $state(false);
	let removeError = $state('');
	let invitationToRevoke = $state<Invitation | null>(null);
	let revokeDialogOpen = $state(false);
	let revokeLoading = $state(false);
	let revokeError = $state('');
	let processingRoleChange = $state<number | null>(null);
	let expandedMemberId = $state<number | null>(null);
	let expandedProjectRoles = $state<MemberProjectRole[]>([]);
	let loadingProjectRoles = $state(false);
	let processingProjectRole = $state<string | null>(null);

	const members = $derived(organizationState.members);
	const invitations = $derived(organizationState.invitations.filter((i) => i.status === 'pending'));
	const canInvite = $derived(organizationState.canInvite);
	const isOwner = $derived(organizationState.isOwner);

	const roleOptions = [
		{ value: 'admin', label: 'Admin' },
		{ value: 'user', label: 'User' },
		{ value: 'readonly', label: 'Read Only' }
	];

	async function handleRoleChange(userId: number, newRole: string) {
		processingRoleChange = userId;
		try {
			await organizationState.updateMemberRole(organizationId, userId, newRole);
			toast.success('Successfully updated the Role', { position: 'top-center' });
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update role');
		} finally {
			processingRoleChange = null;
		}
	}

	async function handleRemoveMember() {
		if (!memberToRemove) return;

		removeLoading = true;
		removeError = '';
		try {
			await organizationState.removeMember(organizationId, memberToRemove.id);
			toast.success('Successfully removed the Member');
			removeDialogOpen = false;
			memberToRemove = null;
		} catch (e) {
			removeError = e instanceof Error ? e.message : 'Failed to remove member';
		} finally {
			removeLoading = false;
		}
	}

	async function handleRevokeInvitation() {
		if (!invitationToRevoke) return;

		revokeLoading = true;
		revokeError = '';
		try {
			await organizationState.revokeInvitation(organizationId, invitationToRevoke.id);
			toast.success('Successfully revoked the Invitation');
			revokeDialogOpen = false;
			invitationToRevoke = null;
		} catch (e) {
			revokeError = e instanceof Error ? e.message : 'Failed to revoke invitation';
		} finally {
			revokeLoading = false;
		}
	}

	function canChangeRole(member: OrganizationMember): boolean {
		if (member.role === 'owner') return false;
		return true;
	}

	function canExpand(member: OrganizationMember): boolean {
		return member.role === 'user' || member.role === 'readonly';
	}

	async function toggleExpand(member: OrganizationMember) {
		if (expandedMemberId === member.id) {
			expandedMemberId = null;
			return;
		}
		expandedMemberId = member.id;
		loadingProjectRoles = true;
		expandedProjectRoles = [];
		try {
			const response = await organizationState.getMemberProjectRoles(organizationId, member.id);
			if (expandedMemberId !== member.id) return;
			expandedProjectRoles = response.projectRoles;
			loadingProjectRoles = false;
		} catch (e) {
			if (expandedMemberId !== member.id) return;
			loadingProjectRoles = false;
			toast.error(e instanceof Error ? e.message : 'Failed to load project roles');
			expandedMemberId = null;
		}
	}

	function projectRoleLabel(member: OrganizationMember, role: string | null): string {
		if (!role || role === 'default') {
			return `Default (${member.role === 'readonly' ? 'Read Only' : 'User'})`;
		}
		return role === 'readonly' ? 'Read Only' : 'User';
	}

	async function handleProjectRoleChange(
		member: OrganizationMember,
		projectRole: MemberProjectRole,
		newRole: string
	) {
		processingProjectRole = projectRole.projectId;
		try {
			await organizationState.updateMemberProjectRole(
				organizationId,
				member.id,
				projectRole.projectId,
				newRole
			);
			projectRole.role = newRole === 'default' ? null : newRole;
			toast.success('Successfully updated the Project Role', { position: 'top-center' });
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update project role');
		} finally {
			processingProjectRole = null;
		}
	}

	function canRemoveMember(member: OrganizationMember): boolean {
		return member.role !== 'owner';
	}

	function getRoleBadgeVariant(role: string): 'default' | 'secondary' | 'destructive' | 'outline' {
		switch (role) {
			case 'owner':
				return 'default';
			case 'admin':
				return 'secondary';
			case 'readonly':
				return 'outline';
			default:
				return 'outline';
		}
	}
</script>

<div class="space-y-6">
	<Card class="pb-0">
		<CardHeader
			class="flex flex-row flex-wrap items-start justify-between gap-4 space-y-0 gap-y-2 pb-4"
		>
			<div class="min-w-0 space-y-1.5">
				<CardTitle>Team Members</CardTitle>
				<CardDescription>Manage your organization's team members</CardDescription>
			</div>
			<Button
				variant="success"
				class="shrink-0"
				onclick={() => (showInviteDialog = true)}
				disabled={!canInvite}
			>
				<Plus class="mr-2 h-4 w-4" />
				New Invitation
			</Button>
		</CardHeader>
		<CardContent class="p-0">
			<TableContainer class="rounded-none border-0">
				<Table.Root>
					<Table.Header>
						<Table.Row class="hover:bg-transparent">
							<Table.Head>Name</Table.Head>
							<Table.Head>Email</Table.Head>
							<Table.Head>Role</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head class="w-[100px]"></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each members as member (member.id)}
							<Table.Row>
								<Table.Cell class="font-medium">
									<div class="flex items-center gap-1">
										{#if canExpand(member)}
											<button
												type="button"
												class="rounded p-0.5 hover:bg-accent"
												title="Per-project access"
												onclick={() => toggleExpand(member)}
											>
												{#if expandedMemberId === member.id}
													<ChevronDown class="h-4 w-4" />
												{:else}
													<ChevronRight class="h-4 w-4" />
												{/if}
											</button>
										{:else}
											<span class="inline-block w-5"></span>
										{/if}
										{member.name}
									</div>
								</Table.Cell>
								<Table.Cell>{member.email}</Table.Cell>
								<Table.Cell>
									{#if canChangeRole(member) && member.role !== 'owner'}
										<Select.Root
											type="single"
											value={member.role}
											onValueChange={(val) => val && handleRoleChange(member.id, val)}
											disabled={processingRoleChange === member.id ||
												(!isOwner && member.role === 'admin')}
										>
											<Select.Trigger class="w-[130px]">
												{roleOptions.find((r) => r.value === member.role)?.label || member.role}
											</Select.Trigger>
											<Select.Content>
												{#each roleOptions as role, __index (__index)}
													{#if isOwner || role.value !== 'admin'}
														<Select.Item value={role.value}>
															{role.label}
														</Select.Item>
													{/if}
												{/each}
											</Select.Content>
										</Select.Root>
									{:else}
										<Badge variant={getRoleBadgeVariant(member.role)}>
											{member.role === 'owner'
												? 'Owner'
												: member.role === 'admin'
													? 'Admin'
													: member.role === 'readonly'
														? 'Read Only'
														: 'User'}
										</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell>
									<StatusPill tone="success" label="Active" />
								</Table.Cell>
								<Table.Cell>
									{#if canRemoveMember(member)}
										<Button
											variant="ghost"
											size="icon"
											onclick={() => {
												removeError = '';
												memberToRemove = member;
												removeDialogOpen = true;
											}}
										>
											<Trash2 class="h-4 w-4 text-destructive" />
										</Button>
									{/if}
								</Table.Cell>
							</Table.Row>
							{#if expandedMemberId === member.id && canExpand(member)}
								<Table.Row class="bg-muted/30 hover:bg-transparent">
									<Table.Cell colspan={5} class="py-2">
										{#if loadingProjectRoles}
											<div class="flex justify-center py-4">
												<LoadingCircle />
											</div>
										{:else}
											<div class="space-y-1 pr-2 pl-10">
												{#each expandedProjectRoles as projectRole (projectRole.projectId)}
													<div class="flex items-center justify-between gap-4 py-1">
														<div class="flex items-center gap-2">
															<FrameworkIcon
																framework={projectRole.framework as Framework}
																class="size-4"
															/>
															<span class="text-sm">{projectRole.name}</span>
														</div>
														<Select.Root
															type="single"
															value={projectRole.role ?? 'default'}
															onValueChange={(val) =>
																val && handleProjectRoleChange(member, projectRole, val)}
															disabled={processingProjectRole === projectRole.projectId}
														>
															<Select.Trigger class="w-[200px]">
																{projectRoleLabel(member, projectRole.role)}
															</Select.Trigger>
															<Select.Content>
																<Select.Item value="default"
																	>Default ({member.role === 'readonly'
																		? 'Read Only'
																		: 'User'})</Select.Item
																>
																<Select.Item value="user">User</Select.Item>
																<Select.Item value="readonly">Read Only</Select.Item>
															</Select.Content>
														</Select.Root>
													</div>
												{:else}
													<div class="py-2 text-sm text-muted-foreground">
														No projects in this organization yet
													</div>
												{/each}
											</div>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/if}
						{:else}
							<TableEmptyState colspan={5} message="No members yet." />
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		</CardContent>
	</Card>

	<Card class="pb-0">
		<CardHeader>
			<CardTitle>Pending Invitations</CardTitle>
			<CardDescription>Invitations that have not yet been accepted</CardDescription>
		</CardHeader>
		<CardContent class="p-0">
			<TableContainer class="rounded-none border-0">
				<Table.Root>
					<Table.Header>
						<Table.Row class="hover:bg-transparent">
							<Table.Head>Email</Table.Head>
							<Table.Head>Role</Table.Head>
							<Table.Head>Invited By</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head class="w-[100px]"></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each invitations as invitation, __index (__index)}
							<Table.Row>
								<Table.Cell class="font-medium">{invitation.email}</Table.Cell>
								<Table.Cell>
									<Badge variant="outline">
										{invitation.role === 'admin'
											? 'Admin'
											: invitation.role === 'readonly'
												? 'Read Only'
												: 'User'}
									</Badge>
								</Table.Cell>
								<Table.Cell>{invitation.inviterName}</Table.Cell>
								<Table.Cell>
									<StatusPill tone="warning" label="Invited" />
								</Table.Cell>
								<Table.Cell>
									<Button
										variant="ghost"
										size="icon"
										onclick={() => {
											revokeError = '';
											invitationToRevoke = invitation;
											revokeDialogOpen = true;
										}}
									>
										<Trash2 class="h-4 w-4 text-destructive" />
									</Button>
								</Table.Cell>
							</Table.Row>
						{:else}
							<TableEmptyState colspan={5} message="No pending invitations." />
						{/each}
					</Table.Body>
				</Table.Root>
			</TableContainer>
		</CardContent>
	</Card>
</div>

<InviteUserDialog bind:open={showInviteDialog} {organizationId} />

<ConfirmDeleteDialog
	bind:open={removeDialogOpen}
	entity="Member"
	verb="Remove"
	title="Remove Team Member"
	description={`Are you sure you want to remove ${memberToRemove?.name} from this organization? They will lose access to all projects.`}
	error={removeError}
	loading={removeLoading}
	onConfirm={handleRemoveMember}
/>

<ConfirmDeleteDialog
	bind:open={revokeDialogOpen}
	entity="Invitation"
	verb="Revoke"
	description={`Are you sure you want to revoke the invitation for ${invitationToRevoke?.email}?`}
	error={revokeError}
	loading={revokeLoading}
	onConfirm={handleRevokeInvitation}
/>
