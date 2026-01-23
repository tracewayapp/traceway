<script lang="ts">
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert, CircleCheck } from "@lucide/svelte";
    import { themeState } from '$lib/state/theme.svelte';

    let email = $state('');
    let error = $state('');
    let loading = $state(false);
    let success = $state(false);

    async function handleSubmit() {
        loading = true;
        error = '';

        try {
            const response = await fetch('/api/forgot-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ email })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to process request');
            }

            success = true;
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
            {#if success}
                <Alert class="mb-4 bg-green-50 border-green-200">
                    <CircleCheck class="h-4 w-4 text-green-700" />
                    <AlertTitle class="text-green-800">Check your email</AlertTitle>
                    <AlertDescription class="text-green-700">
                        If an account exists with this email, a password reset link will be sent to it.
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
                    Enter your email address and we'll send you a link to reset your password.
                </p>
                <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="grid w-full items-center gap-4">
                    <div class="flex flex-col space-y-1.5">
                        <Label for="email">Email</Label>
                        <Input id="email" type="email" bind:value={email} placeholder="you@example.com" required />
                    </div>
                    <Button type="submit" disabled={loading} class="w-full">
                        {#if loading}
                            Sending...
                        {:else}
                            Send Reset Link
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
