<script lang="ts">
    import * as AlertDialog from "$lib/components/ui/alert-dialog";
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import * as Select from "$lib/components/ui/select";
    import { UserPlus } from "@lucide/svelte";
    import { toast } from 'svelte-sonner';
    import { organizationState } from '$lib/state/organization.svelte';

    interface Props {
        open: boolean;
        organizationId: number;
    }

    let { open = $bindable(), organizationId }: Props = $props();

    let email = $state('');
    let role = $state('user');
    let loading = $state(false);
    let error = $state('');

    const roleOptions = [
        { value: 'admin', label: 'Admin', description: 'Full access to projects and settings' },
        { value: 'user', label: 'User', description: 'Can view and modify projects' },
        { value: 'readonly', label: 'Read Only', description: 'Can only view projects' }
    ];

    async function handleSubmit() {
        if (!email) {
            error = 'Email is required';
            return;
        }

        loading = true;
        error = '';

        try {
            await organizationState.inviteUser(organizationId, email, role);
            toast.success('Invitation sent', { position: 'top-center' });
            email = '';
            role = 'user';
            open = false;
        } catch (e: unknown) {
            error = e instanceof Error ? e.message : 'Failed to send invitation';
            toast.error(error);
        } finally {
            loading = false;
        }
    }

    function handleOpenChange(isOpen: boolean) {
        if (!isOpen) {
            email = '';
            role = 'user';
            error = '';
        }
        open = isOpen;
    }
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
    <AlertDialog.Content class="max-w-md">
        <AlertDialog.Header>
            <AlertDialog.Title>Invite Team Member</AlertDialog.Title>
            <AlertDialog.Description>
                Send an invitation to join your organization
            </AlertDialog.Description>
        </AlertDialog.Header>

        <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4 py-4">
            <div class="space-y-2">
                <Label for="email">Email Address</Label>
                <Input
                    id="email"
                    type="email"
                    bind:value={email}
                    placeholder="colleague@example.com"
                    required
                />
            </div>

            <div class="space-y-2">
                <Label for="role">Role</Label>
                <Select.Root type="single" bind:value={role}>
                    <Select.Trigger class="w-full">
                        {roleOptions.find(r => r.value === role)?.label || 'Select role'}
                    </Select.Trigger>
                    <Select.Content>
                        {#each roleOptions as roleOption}
                            <Select.Item value={roleOption.value}>
                                <div class="flex flex-col">
                                    <span>{roleOption.label}</span>
                                    <span class="text-xs text-muted-foreground">{roleOption.description}</span>
                                </div>
                            </Select.Item>
                        {/each}
                    </Select.Content>
                </Select.Root>
            </div>

            {#if error}
                <p class="text-sm text-destructive">{error}</p>
            {/if}
        </form>

        <AlertDialog.Footer>
            <AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
            <Button onclick={handleSubmit} disabled={loading}>
                <UserPlus class="mr-2 h-4 w-4" />
                {#if loading}
                    Inviting...
                {:else}
                    Invite User
                {/if}
            </Button>
        </AlertDialog.Footer>
    </AlertDialog.Content>
</AlertDialog.Root>
