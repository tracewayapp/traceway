<script lang="ts">
    import { Button } from "$lib/components/ui/button";
    import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "$lib/components/ui/card";
    import * as Table from "$lib/components/ui/table";
    import * as Select from "$lib/components/ui/select";
    import * as AlertDialog from "$lib/components/ui/alert-dialog";
    import { Badge } from "$lib/components/ui/badge";
    import { UserPlus, Trash2 } from "@lucide/svelte";
    import { toast } from 'svelte-sonner';
    import { organizationState, type OrganizationMember, type Invitation } from '$lib/state/organization.svelte';
    import { authState } from '$lib/state/auth.svelte';
    import InviteUserDialog from './invite-user-dialog.svelte';

    interface Props {
        organizationId: number;
    }

    let { organizationId }: Props = $props();

    let showInviteDialog = $state(false);
    let memberToRemove = $state<OrganizationMember | null>(null);
    let invitationToRevoke = $state<Invitation | null>(null);
    let processingRoleChange = $state<number | null>(null);

    const members = $derived(organizationState.members);
    const invitations = $derived(organizationState.invitations.filter(i => i.status === 'pending'));
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
            toast.success('Role updated', { position: 'top-center' });
        } catch (e) {
            toast.error(e instanceof Error ? e.message : 'Failed to update role', { position: 'top-center' });
        } finally {
            processingRoleChange = null;
        }
    }

    async function handleRemoveMember() {
        if (!memberToRemove) return;

        try {
            await organizationState.removeMember(organizationId, memberToRemove.id);
            toast.success('Successfully removed the Member', { position: 'top-center' });
        } catch (e) {
            toast.error(e instanceof Error ? e.message : 'Failed to remove member');
        } finally {
            memberToRemove = null;
        }
    }

    async function handleRevokeInvitation() {
        if (!invitationToRevoke) return;

        try {
            await organizationState.revokeInvitation(organizationId, invitationToRevoke.id);
            toast.success('Successfully revoked the Invitation', { position: 'top-center' });
        } catch (e) {
            toast.error(e instanceof Error ? e.message : 'Failed to revoke invitation');
        } finally {
            invitationToRevoke = null;
        }
    }

    function canChangeRole(member: OrganizationMember): boolean {
        if (member.role === 'owner') return false;
        return true;
    }

    function canRemoveMember(member: OrganizationMember): boolean {
        return member.role !== 'owner';
    }

    function getRoleBadgeVariant(role: string): 'default' | 'secondary' | 'destructive' | 'outline' {
        switch (role) {
            case 'owner': return 'default';
            case 'admin': return 'secondary';
            case 'readonly': return 'outline';
            default: return 'outline';
        }
    }
</script>

<div class="space-y-6">
    <Card class="pb-0">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-4">
            <div>
                <CardTitle>Team Members</CardTitle>
                <CardDescription>Manage your organization's team members</CardDescription>
            </div>
            <Button variant="outline" onclick={() => showInviteDialog = true} disabled={!canInvite}>
                <UserPlus class="mr-2 h-4 w-4" />
                Invite User
            </Button>
        </CardHeader>
        <CardContent class="p-0">
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
                    {#each members as member}
                        <Table.Row>
                            <Table.Cell class="font-medium">{member.name}</Table.Cell>
                            <Table.Cell>{member.email}</Table.Cell>
                            <Table.Cell>
                                {#if canChangeRole(member) && member.role !== 'owner'}
                                    <Select.Root
                                        type="single"
                                        value={member.role}
                                        onValueChange={(val) => val && handleRoleChange(member.id, val)}
                                        disabled={processingRoleChange === member.id || (!isOwner && member.role === 'admin')}
                                    >
                                        <Select.Trigger class="w-[130px]">
                                            {roleOptions.find(r => r.value === member.role)?.label || member.role}
                                        </Select.Trigger>
                                        <Select.Content>
                                            {#each roleOptions as role}
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
                                        {member.role === 'owner' ? 'Owner' :
                                         member.role === 'admin' ? 'Admin' :
                                         member.role === 'readonly' ? 'Read Only' : 'User'}
                                    </Badge>
                                {/if}
                            </Table.Cell>
                            <Table.Cell>
                                <Badge variant="outline" class="bg-green-50 text-green-700 border-green-200">
                                    Active
                                </Badge>
                            </Table.Cell>
                            <Table.Cell>
                                {#if canRemoveMember(member)}
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        onclick={() => memberToRemove = member}
                                    >
                                        <Trash2 class="h-4 w-4 text-destructive" />
                                    </Button>
                                {/if}
                            </Table.Cell>
                        </Table.Row>
                    {/each}
                </Table.Body>
            </Table.Root>
        </CardContent>
    </Card>

    {#if invitations.length > 0}
        <Card class="pb-0">
            <CardHeader>
                <CardTitle>Pending Invitations</CardTitle>
                <CardDescription>Invitations that have not yet been accepted</CardDescription>
            </CardHeader>
            <CardContent class="p-0">
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
                        {#each invitations as invitation}
                            <Table.Row>
                                <Table.Cell class="font-medium">{invitation.email}</Table.Cell>
                                <Table.Cell>
                                    <Badge variant="outline">
                                        {invitation.role === 'admin' ? 'Admin' :
                                         invitation.role === 'readonly' ? 'Read Only' : 'User'}
                                    </Badge>
                                </Table.Cell>
                                <Table.Cell>{invitation.inviterName}</Table.Cell>
                                <Table.Cell>
                                    <Badge variant="outline" class="bg-yellow-50 text-yellow-700 border-yellow-200">
                                        Invited
                                    </Badge>
                                </Table.Cell>
                                <Table.Cell>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        onclick={() => invitationToRevoke = invitation}
                                    >
                                        <Trash2 class="h-4 w-4 text-destructive" />
                                    </Button>
                                </Table.Cell>
                            </Table.Row>
                        {/each}
                    </Table.Body>
                </Table.Root>
            </CardContent>
        </Card>
    {/if}
</div>

<InviteUserDialog
    bind:open={showInviteDialog}
    {organizationId}
/>

<AlertDialog.Root open={memberToRemove !== null} onOpenChange={(open) => { if (!open) memberToRemove = null; }}>
    <AlertDialog.Content>
        <AlertDialog.Header>
            <AlertDialog.Title>Remove Team Member</AlertDialog.Title>
            <AlertDialog.Description>
                Are you sure you want to remove {memberToRemove?.name} from this organization?
                They will lose access to all projects.
            </AlertDialog.Description>
        </AlertDialog.Header>
        <AlertDialog.Footer>
            <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
            <AlertDialog.Action onclick={handleRemoveMember}>
                Remove
            </AlertDialog.Action>
        </AlertDialog.Footer>
    </AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root open={invitationToRevoke !== null} onOpenChange={(open) => { if (!open) invitationToRevoke = null; }}>
    <AlertDialog.Content>
        <AlertDialog.Header>
            <AlertDialog.Title>Revoke Invitation</AlertDialog.Title>
            <AlertDialog.Description>
                Are you sure you want to revoke the invitation for {invitationToRevoke?.email}?
            </AlertDialog.Description>
        </AlertDialog.Header>
        <AlertDialog.Footer>
            <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
            <AlertDialog.Action onclick={handleRevokeInvitation}>
                Revoke
            </AlertDialog.Action>
        </AlertDialog.Footer>
    </AlertDialog.Content>
</AlertDialog.Root>
