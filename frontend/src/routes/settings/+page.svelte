<script lang="ts">
    import { goto } from '$app/navigation';
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState } from '$lib/state/projects.svelte';
    import { organizationState } from '$lib/state/organization.svelte';
    import OrganizationTab from './organization-tab.svelte';
    import UsersTab from './users-tab.svelte';
    import type { Component } from 'svelte';
    import { LoadingCircle } from '$lib/components/ui/loading-circle';

    let loading = $state(true);
    let error = $state<string | null>(null);
    let BillingTab = $state<Component<{ organizationId: number }> | null>(null);

    async function loadBillingModule() {
        try {
            // @ts-ignore - $billing alias only exists when billing extension is available
            const module = await import('$billing/billing-tab.svelte');
            BillingTab = module.default;
        } catch {
            // Billing extension not available - this is expected for open source builds
        }
    }

    $effect(() => {
        loadBillingModule();
    });

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

<div class="space-y-4">
    <div>
        <h1 class="text-3xl font-semibold tracking-tight">Settings</h1>
    </div>

    {#if loading}
        <div class="flex items-center justify-center py-12">
            <LoadingCircle size="xlg" />
        </div>
    {:else if error}
        <div class="text-center py-12 text-destructive">
            {error}
        </div>
    {:else}
        <div class="space-y-4">
            <OrganizationTab />
            <UsersTab organizationId={currentOrganizationId!} />
            {#if BillingTab}
                <BillingTab organizationId={currentOrganizationId!} />
            {/if}
        </div>
    {/if}
</div>
