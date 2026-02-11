<script lang="ts">
    import { goto } from '$app/navigation';
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Label } from "$lib/components/ui/label";
    import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "$lib/components/ui/card";
    import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
    import * as Select from "$lib/components/ui/select";
    import { CircleAlert, Check } from "@lucide/svelte";
    import { authState } from '$lib/state/auth.svelte';
    import { projectsState, type Framework } from '$lib/state/projects.svelte';
    import { themeState } from '$lib/state/theme.svelte';
    import { toast } from 'svelte-sonner';
    import FrameworkCombobox from '$lib/components/framework-combobox.svelte';
    import TurnstileWidget from '$lib/components/turnstile-widget.svelte';

    let email = $state('');
    let name = $state('');
    let password = $state('');
    let confirmPassword = $state('');
    let organizationName = $state('');
    let timezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone);
    let projectName = $state('');
    let framework = $state<Framework>('gin');
    let error = $state('');
    let loading = $state(false);
    let captchaToken = $state('');

    const turnstileSiteKey = __TURNSTILE_SITE_KEY__;
    const captchaEnabled = turnstileSiteKey !== '';

    const timezones = Intl.supportedValuesOf('timeZone');

    if (!__CLOUD_MODE__) {
        $effect(() => {
            // if we're not in the cloud mode we have to check if an organization exists and if it does we should go to the login page

            loading = true;
            fetch('/api/has-organizations', {
                method: 'GET',
            })
            .then(response => response.json())
            .then((response) => {
                if (response.hasOrganizations) {
                    goto("/login")
                }
                loading = false
            }).catch(() => {
                toast.error("An unexpected error has occured. The page will refresh in 5 seconds.")
                setTimeout(() => {
                    window.location.reload()
                }, 5000)
            });
        })
    }

    async function handleRegister() {
        if (password !== confirmPassword) {
            error = 'Passwords do not match';
            return;
        }

        if (password.length < 8) {
            error = 'Password must be at least 8 characters';
            return;
        }

        if (captchaEnabled && !captchaToken) {
            error = 'Please complete the captcha';
            return;
        }

        loading = true;
        error = '';

        try {
            const response = await fetch('/api/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    email,
                    name,
                    password,
                    organizationName,
                    timezone,
                    projectName,
                    framework,
                    captchaToken
                })
            });

            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || 'Registration failed');
            }

            const data = await response.json();

            authState.setToken(data.token);
            authState.setOrganizations(data.organizations || []);
            projectsState.setProjects(data.projects);

            goto('/');
        } catch (e) {
            error = e instanceof Error ? e.message : 'Registration failed';
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
                Create your account
            </CardDescription>
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
            <form onsubmit={(e) => { e.preventDefault(); handleRegister(); }} class="grid w-full items-center gap-4">
                <div class="flex flex-col space-y-1.5">
                    <Label for="email">Email</Label>
                    <Input id="email" type="email" bind:value={email} placeholder="you@example.com" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="name">Name</Label>
                    <Input id="name" type="text" bind:value={name} placeholder="Your name" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="password">Password</Label>
                    <Input id="password" type="password" bind:value={password} placeholder="Password (min 8 characters)" required />
                </div>
                <div class="flex flex-col space-y-1.5">
                    <Label for="confirmPassword">Confirm Password</Label>
                    <Input id="confirmPassword" type="password" bind:value={confirmPassword} placeholder="Confirm password" required />
                </div>

                <div class="flex items-center gap-3 mt-2">
                    <div class="flex-1 border-t"></div>
                    <p class="text-sm text-muted-foreground">Organization & Project</p>
                    <div class="flex-1 border-t"></div>
                </div>

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

                {#if captchaEnabled}
                    <div class="flex flex-col space-y-1.5 mt-2">
                        <TurnstileWidget
                            siteKey={turnstileSiteKey}
                            onVerify={(token) => captchaToken = token}
                            onError={() => captchaToken = ''}
                        />
                    </div>
                {/if}

                <Button type="submit" disabled={loading || (captchaEnabled && !captchaToken)} class="w-full mt-2">
                    {#if loading}
                        Creating account...
                    {:else}
                        Create Account
                    {/if}
                </Button>
            </form>
        </CardContent>

        {#if __CLOUD_MODE__}
            <CardFooter class="flex flex-col justify-center">
                <p class="text-sm text-muted-foreground">
                    Already have an account? <a href="/login" class="text-primary hover:underline">Login</a>
                </p>
            </CardFooter>
        {/if}
    </Card>
</div>
