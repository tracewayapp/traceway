<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/state';
    import { onMount } from 'svelte';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import { CircleAlert, Check } from "@lucide/svelte";
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState } from '$lib/state/projects.svelte';
    import { themeState } from '$lib/state/theme.svelte';

    interface InvitationInfo {
        email: string;
        organizationName: string;
        inviterName: string;
        existsAsUser: boolean;
        role: string;
    }

    let loading = $state(true);
    let submitting = $state(false);
    let error = $state('');
    let invitationInfo = $state<InvitationInfo | null>(null);
    let name = $state('');
    let password = $state('');
    let confirmPassword = $state('');
    let success = $state(false);

    const token = $derived(page.url.searchParams.get('token'));

    onMount(async () => {
        if (!token) {
            error = 'Invalid invitation link';
            loading = false;
            return;
        }

        try {
            const response = await fetch(`/api/invitations/${token}`);

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Invalid or expired invitation');
            }

            invitationInfo = await response.json();
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load invitation';
        } finally {
            loading = false;
        }
    });

    async function handleNewUserSubmit() {
        if (password !== confirmPassword) {
            error = 'Passwords do not match';
            return;
        }

        if (password.length < 8) {
            error = 'Password must be at least 8 characters';
            return;
        }

        submitting = true;
        error = '';

        try {
            const response = await fetch(`/api/invitations/${token}/accept`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, password })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to accept invitation');
            }

            const data = await response.json();

            authState.setToken(data.token);
            authState.setOrganizations(data.organizations || []);
            projectsState.setProjects(data.projects);

            success = true;
            setTimeout(() => goto('/'), 1500);
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to accept invitation';
        } finally {
            submitting = false;
        }
    }

    async function handleExistingUserAccept() {
        submitting = true;
        error = '';

        try {
            const response = await fetch(`/api/invitations/${token}/accept-existing`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${authState.token}`
                }
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Failed to accept invitation');
            }

            const data = await response.json();
            projectsState.setProjects(data.projects);
            success = true;
            setTimeout(() => goto('/'), 1500);
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to accept invitation';
        } finally {
            submitting = false;
        }
    }

    function handleLoginRedirect() {
        const returnTo = encodeURIComponent(`/accept-invitation?token=${token}`);
        goto(`/login?returnTo=${returnTo}`);
    }

    function getRoleLabel(role: string): string {
        switch (role) {
            case 'admin': return 'Admin';
            case 'readonly': return 'Read Only';
            default: return 'Member';
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
            {#if invitationInfo && !success}
                <CardDescription class="text-center">
                    You've been invited to join <strong>{invitationInfo.organizationName}</strong>
                </CardDescription>
            {/if}
        </CardHeader>
        <CardContent>
            {#if loading}
                <div class="flex items-center justify-center py-8">
                    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                </div>
            {:else if success}
                <div class="flex flex-col items-center justify-center py-8 gap-4">
                    <div class="rounded-full bg-green-100 p-3">
                        <Check class="h-8 w-8 text-green-600" />
                    </div>
                    <p class="text-center text-lg font-medium">Welcome to {invitationInfo?.organizationName}!</p>
                    <p class="text-center text-muted-foreground">Redirecting to dashboard...</p>
                </div>
            {:else if error && !invitationInfo}
                <Alert variant="destructive" class="bg-red-50 border-red-200">
                    <CircleAlert class="h-4 w-4 text-red-700" />
                    <AlertTitle class="text-red-800">Error</AlertTitle>
                    <AlertDescription class="text-red-700">
                        {error}
                    </AlertDescription>
                </Alert>
                <div class="mt-4 text-center">
                    <Button variant="outline" onclick={() => goto('/login')}>
                        Go to Login
                    </Button>
                </div>
            {:else if invitationInfo}
                <div class="space-y-4">
                    <div class="bg-muted/50 rounded-lg p-4 text-sm">
                        <p><strong>{invitationInfo.inviterName}</strong> invited you to join as a <strong>{getRoleLabel(invitationInfo.role)}</strong></p>
                    </div>

                    {#if error}
                        <Alert variant="destructive" class="bg-red-50 border-red-200">
                            <CircleAlert class="h-4 w-4 text-red-700" />
                            <AlertDescription class="text-red-700">
                                {error}
                            </AlertDescription>
                        </Alert>
                    {/if}

                    {#if invitationInfo.existsAsUser}
                        {#if authState.isAuthenticated}
                            <p class="text-center text-muted-foreground">
                                Click below to join the organization
                            </p>
                            <Button class="w-full" onclick={handleExistingUserAccept} disabled={submitting}>
                                {#if submitting}
                                    Joining...
                                {:else}
                                    Accept Invitation
                                {/if}
                            </Button>
                            <div>
                                <p class="text-sm text-muted-foreground">
                                    To go to the Login page <a href="/login" class="text-primary hover:underline">Click Here</a>
                                </p>
                            </div>
                        {:else}
                            <p class="text-center text-muted-foreground">
                                You already have an account. Please log in to accept this invitation.
                            </p>
                            <Button class="w-full" onclick={handleLoginRedirect}>
                                Login to Accept
                            </Button>
                        {/if}
                    {:else}
                        <form onsubmit={(e) => { e.preventDefault(); handleNewUserSubmit(); }} class="space-y-4">
                            <div class="space-y-2">
                                <Label for="email">Email</Label>
                                <Input id="email" type="email" value={invitationInfo.email} disabled />
                            </div>
                            <div class="space-y-2">
                                <Label for="name">Your Name</Label>
                                <Input id="name" type="text" bind:value={name} placeholder="Your name" required />
                            </div>
                            <div class="space-y-2">
                                <Label for="password">Password</Label>
                                <Input id="password" type="password" bind:value={password} placeholder="Min 8 characters" required />
                            </div>
                            <div class="space-y-2">
                                <Label for="confirmPassword">Confirm Password</Label>
                                <Input id="confirmPassword" type="password" bind:value={confirmPassword} placeholder="Confirm password" required />
                            </div>
                            <Button type="submit" class="w-full" disabled={submitting}>
                                {#if submitting}
                                    Creating account...
                                {:else}
                                    Create Account & Join
                                {/if}
                            </Button>
                        </form>
                    {/if}
                </div>
            {/if}
        </CardContent>
    </Card>
</div>
