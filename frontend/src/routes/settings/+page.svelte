<script lang="ts">
    import { goto } from '$app/navigation';
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState } from '$lib/state/projects.svelte';
    import { organizationState } from '$lib/state/organization.svelte';
    import OrganizationTab from './organization-tab.svelte';
    import UsersTab from './users-tab.svelte';

    let loading = $state(true);
    let error = $state<string | null>(null);

    const currentOrganizationId = $derived(projectsState.currentProject?.organizationId);

    const hasAccess = $derived(
        currentOrganizationId !== null &&
        currentOrganizationId !== undefined &&
        authState.canManageOrganization(currentOrganizationId)
    );

    $effect(() => {
        if (!hasAccess && !loading) {
            goto('/');
        }
    });

    $effect(() => {
        if (currentOrganizationId && hasAccess) {
            loading = true;
            organizationState.loadSettings(currentOrganizationId)
                .catch(e => {
                    error = e instanceof Error ? e.message : 'Failed to load settings';
                })
                .finally(() => {
                    loading = false;
                });
        }
    });
</script>

<div class="space-y-6">
    <div>
        <h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
    </div>

    {#if loading}
        <div class="flex items-center justify-center py-12">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
        </div>
    {:else if error}
        <div class="text-center py-12 text-destructive">
            {error}
        </div>
    {:else}
        <div class="space-y-6">
            <OrganizationTab />
            <UsersTab organizationId={currentOrganizationId!} />
        </div>
    {/if}
</div>
