<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/state';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert, CircleCheck, Loader2 } from "@lucide/svelte";
    import { themeState } from '$lib/state/theme.svelte';

    let password = $state('');
    let confirmPassword = $state('');
    let error = $state('');
    let loading = $state(false);
    let success = $state(false);
    let validating = $state(true);
    let tokenValid = $state(false);
    let email = $state('');

    const token = $derived(page.url.searchParams.get('token'));

    $effect(() => {
        if (token) {
            validateToken(token);
        } else {
            validating = false;
            tokenValid = false;
        }
    });

    async function validateToken(token: string) {
        try {
            const response = await fetch(`/api/password-reset/${token}`);
            const data = await response.json();

            if (data.valid) {
                tokenValid = true;
                email = data.email;
            } else {
                tokenValid = false;
            }
        } catch {
            tokenValid = false;
        } finally {
            validating = false;
        }
    }

    async function handleSubmit() {
        if (password !== confirmPassword) {
            error = 'Passwords do not match';
            return;
        }

        if (password.length < 8) {
            error = 'Password must be at least 8 characters';
            return;
        }

        loading = true;
        error = '';

        try {
            const response = await fetch(`/api/password-reset/${token}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ password })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to reset password');
            }

            success = true;
            setTimeout(() => {
                goto('/login');
            }, 2000);
        } catch (e) {
            error = e instanceof Error ? e.message : 'An unexpected error occurred';
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
            {#if validating}
                <div class="flex flex-col items-center justify-center py-8">
                    <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
                    <p class="mt-4 text-sm text-muted-foreground">Validating reset link...</p>
                </div>
            {:else if !tokenValid}
                <Alert variant="destructive" class="bg-red-50 border-red-200">
                    <CircleAlert class="h-4 w-4 text-red-700" />
                    <AlertTitle class="text-red-800">Invalid Link</AlertTitle>
                    <AlertDescription class="text-red-700">
                        This password reset link is invalid or has expired. Please request a new one.
                    </AlertDescription>
                </Alert>
            {:else if success}
                <Alert class="bg-green-50 border-green-200">
                    <CircleCheck class="h-4 w-4 text-green-700" />
                    <AlertTitle class="text-green-800">Password Reset</AlertTitle>
                    <AlertDescription class="text-green-700">
                        Your password has been reset successfully. Redirecting to login...
                    </AlertDescription>
                </Alert>
            {:else}
                {#if error}
                    <Alert variant="destructive" class="mb-4 bg-red-50 border-red-200">
                        <CircleAlert class="h-4 w-4 text-red-700" />
                        <AlertTitle class="text-red-800">Error</AlertTitle>
                        <AlertDescription class="text-red-700">
                            {error}
                        </AlertDescription>
                    </Alert>
                {/if}
                <p class="text-sm text-muted-foreground mb-4">
                    Enter a new password for your account.
                </p>
                <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="grid w-full items-center gap-4">
                    <div class="flex flex-col space-y-1.5">
                        <Label for="email">Email</Label>
                        <Input id="email" type="email" value={email} disabled class="bg-muted" />
                    </div>
                    <div class="flex flex-col space-y-1.5">
                        <Label for="password">New Password</Label>
                        <Input id="password" type="password" bind:value={password} placeholder="New password" required minlength={8} />
                    </div>
                    <div class="flex flex-col space-y-1.5">
                        <Label for="confirmPassword">Confirm Password</Label>
                        <Input id="confirmPassword" type="password" bind:value={confirmPassword} placeholder="Confirm password" required minlength={8} />
                    </div>
                    <Button type="submit" disabled={loading} class="w-full">
                        {#if loading}
                            Resetting...
                        {:else}
                            Reset Password
                        {/if}
                    </Button>
                </form>
            {/if}
        </CardContent>
        <CardFooter class="flex flex-col justify-center">
            <p class="text-sm text-muted-foreground">
                <a href="/login" class="text-primary hover:underline">Back to Login</a>
            </p>
        </CardFooter>
    </Card>
</div>
