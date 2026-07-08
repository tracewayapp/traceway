<script lang="ts">
    import { goto } from '$app/navigation';
    import { onMount } from 'svelte';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import * as Select from "$lib/components/ui/select";
    import { CircleAlert, Check } from "@lucide/svelte";
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState, type Framework } from '$lib/state/projects.svelte';
    import { themeState } from '$lib/state/theme.svelte';
    import FrameworkCombobox from '$lib/components/framework-combobox.svelte';
    import { consumeSsoReturnTo, safeLocalPath } from '$lib/utils/navigation';

    const DEFAULT_FRAMEWORK: Framework = 'gin';

    let organizationName = $state('');
    let timezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone);
    let projectName = $state('');
    let framework = $state<Framework>(DEFAULT_FRAMEWORK);
    let error = $state('');
    let loading = $state(false);

    const timezones = Intl.supportedValuesOf('timeZone');

    onMount(() => {
        if (!authState.token) {
            goto('/login');
        }
    });

    async function handleSubmit() {
        loading = true;
        error = '';
        try {
            const response = await fetch('/api/auth/finish-setup', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${authState.token}`,
                },
                body: JSON.stringify({ organizationName, timezone, projectName, framework })
            });

            if (!response.ok) {
                const data = await response.json().catch(() => ({}));
                throw new Error(data.error || 'Setup failed');
            }

            const data = await response.json();
            authState.setToken(data.token);
            authState.setOrganizations(data.organizations || []);
            projectsState.setProjects(data.projects);
            goto(safeLocalPath(consumeSsoReturnTo()));
        } catch (e) {
            error = e instanceof Error ? e.message : 'Setup failed';
        } finally {
            loading = false;
        }
    }
</script>

<div class="flex min-h-screen w-full items-center justify-center px-4 py-8">
    <Card class="w-[400px]">
        <CardHeader>
            <CardTitle class="text-2xl">
                <div class="flex flex-row items-center justify-center gap-2">
                    {#if themeState.isDark}
                        <img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-8 w-auto" />
                    {:else}
                        <img src="/traceway-logo.png" alt="Traceway Logo" class="h-8 w-auto" />
                    {/if}
                </div>
            </CardTitle>
            <CardDescription class="text-center">
                Finish setting up your account
            </CardDescription>
        </CardHeader>
        <CardContent>
            {#if error}
                <Alert variant="destructive" class="mb-4 bg-red-50 border-red-200">
                    <CircleAlert class="h-4 w-4 text-red-700" />
                    <AlertTitle class="text-red-800">Error</AlertTitle>
                    <AlertDescription class="text-red-700">{error}</AlertDescription>
                </Alert>
            {/if}
            <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="grid w-full items-center gap-4">
                <div class="flex flex-col space-y-1.5">
                    <Label for="organizationName">Organization Name</Label>
                    <Input id="organizationName" type="text" bind:value={organizationName} placeholder="Your company or team" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="timezone">Timezone</Label>
                    <Select.Root type="single" bind:value={timezone}>
                        <Select.Trigger class="w-full">
                            <span>{timezone}</span>
                        </Select.Trigger>
                        <Select.Content class="max-h-60">
                            {#each timezones as tz}
                                <Select.Item value={tz}>
                                    {#snippet children({ selected })}
                                        <span>{tz}</span>
                                        {#if selected}
                                            <Check class="absolute end-2 size-4" />
                                        {/if}
                                    {/snippet}
                                </Select.Item>
                            {/each}
                        </Select.Content>
                    </Select.Root>
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="projectName">Project Name</Label>
                    <Input id="projectName" type="text" bind:value={projectName} placeholder="My App" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="framework">Framework</Label>
                    <FrameworkCombobox bind:value={framework} />
                </div>
                <Button type="submit" disabled={loading} class="w-full mt-2">
                    {#if loading}
                        Finishing setup...
                    {:else}
                        Finish setup
                    {/if}
                </Button>
            </form>
        </CardContent>
    </Card>
</div>
