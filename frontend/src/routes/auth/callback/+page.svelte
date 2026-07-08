<script lang="ts">
    import { goto } from '$app/navigation';
    import { onMount } from 'svelte';
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState } from '$lib/state/projects.svelte';
    import { consumeSsoReturnTo, safeLocalPath } from '$lib/utils/navigation';
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert } from "@lucide/svelte";
    import { LoadingCircle } from '$lib/components/ui/loading-circle';

    let error = $state('');

    onMount(async () => {
        const fragment = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : '';
        const params = new URLSearchParams(fragment);
        const token = params.get('token');
        const needsSetup = params.get('needsSetup') === 'true';

        history.replaceState(null, '', window.location.pathname);

        if (!token) {
            error = 'Missing authentication token.';
            setTimeout(() => goto('/login?error=oauth_failed'), 1500);
            return;
        }

        authState.setToken(token);

        if (needsSetup) {
            // Leave the stashed returnTo in place: finish-setup consumes it.
            goto('/finish-setup');
            return;
        }

        try {
            const response = await fetch('/api/me/login-bundle', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (!response.ok) {
                throw new Error('Failed to load account');
            }
            const data = await response.json();
            authState.setOrganizations(data.organizations || []);
            projectsState.setProjects(data.projects || []);
            goto(safeLocalPath(consumeSsoReturnTo()));
        } catch {
            authState.logout();
            error = 'Failed to load your account. Please try logging in again.';
            setTimeout(() => goto('/login?error=oauth_failed'), 1500);
        }
    });
</script>

<div class="flex h-screen w-full items-center justify-center px-4">
    {#if error}
        <Alert variant="destructive" class="max-w-md bg-red-50 border-red-200">
            <CircleAlert class="h-4 w-4 text-red-700" />
            <AlertTitle class="text-red-800">Error</AlertTitle>
            <AlertDescription class="text-red-700">{error}</AlertDescription>
        </Alert>
    {:else}
        <LoadingCircle size="xlg" />
    {/if}
</div>
