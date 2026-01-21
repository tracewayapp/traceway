<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert } from "@lucide/svelte";
    import { authState } from '$lib/state/auth.svelte';
    import { userState } from '$lib/state/user.svelte';
    import { projectsState } from '$lib/state/projects.svelte';
    import { themeState } from '$lib/state/theme.svelte';

    let email = $state('');
    let password = $state('');
    let error = $state('');
    let loading = $state(false);

    async function handleLogin() {
        loading = true;
        error = '';
        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ email, password })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Invalid credentials');
            }

            const data = await response.json();

            // Store token and user
            authState.setToken(data.token);
            userState.setUser(data.user);

            // Load projects after successful login
            await projectsState.loadProjects();

            goto('/');
        } catch (e) {
            error = e instanceof Error ? e.message : 'Invalid email or password';
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
            <form onsubmit={(e) => { e.preventDefault(); handleLogin(); }} class="grid w-full items-center gap-4">
                <div class="flex flex-col space-y-1.5">
                    <Label for="email">Email</Label>
                    <Input id="email" type="email" bind:value={email} placeholder="you@example.com" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="password">Password</Label>
                    <Input id="password" type="password" bind:value={password} placeholder="Password" required />
                </div>
                <Button type="submit" disabled={loading} class="w-full">
                    {#if loading}
                        Logging in...
                    {:else}
                        Login
                    {/if}
                </Button>
            </form>
        </CardContent>
        <CardFooter class="flex flex-col justify-center">
            <p class="text-sm text-muted-foreground">
                Don't have an account? <a href="/register" class="text-primary hover:underline">Create account</a>
            </p>
        </CardFooter>
    </Card>
</div>
