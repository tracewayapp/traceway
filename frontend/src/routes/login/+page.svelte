<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert } from "@lucide/svelte";
    import { authState } from '$lib/state/auth.svelte';
    import { themeState } from '$lib/state/theme.svelte';

    let appToken = $state('');
    let error = $state('');
    let loading = $state(false);

    async function handleLogin() {
        loading = true;
        error = '';
        try {
            // Validate against backend
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ token: appToken })
            });

            if (!response.ok) {
                throw new Error('Invalid token');
            }

            // If successful, store token (store handles localStorage)
            authState.setToken(appToken);
            goto('/issues');
        } catch (e) {
            error = 'Invalid APP_TOKEN';
        } finally {
            loading = false;
        }
    }
</script>

<div class="flex h-screen w-full items-center justify-center px-4">
    <Card class="w-[350px]">
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
        </CardHeader>
        <CardContent>
            {#if error}
                <Alert variant="destructive" class="mb-4 bg-red-50 border-red-200">
                    <CircleAlert class="h-4 w-4 text-red-700" />
                    <AlertTitle class="text-red-800">Error</AlertTitle>
                    <AlertDescription class="text-red-700">
                        {error}
                    </AlertDescription>
                </Alert>
            {/if}
            <div class="grid w-full items-center gap-4">
                <div class="flex flex-col space-y-1.5">
                    <Label for="token">Token</Label>
                    <Input id="token" bind:value={appToken} placeholder="APP_TOKEN" />
                </div>
            </div>
        </CardContent>
        <CardFooter class="flex flex-col justify-between gap-2">
            <Button onclick={handleLogin} disabled={loading} class="w-full">
                {#if loading}
                    Logging in...
                {:else}
                    Login
                {/if}
            </Button>
        </CardFooter>
    </Card>
</div>
