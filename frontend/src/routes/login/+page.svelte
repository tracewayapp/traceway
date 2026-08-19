<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { gotoHref } from '$lib/utils/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { themeState } from '$lib/state/theme.svelte';
	import { toast } from 'svelte-sonner';
	import OauthButtons from '$lib/components/oauth-buttons.svelte';
	import { safeLocalPath } from '$lib/utils/navigation';

	const ERROR_MESSAGES: Record<string, string> = {
		oauth_failed: 'Sign-in failed. Please try again.',
		oauth_no_email:
			"We couldn't read an email address from that account. Please make sure it has a verified email and try again.",
		invite_required: 'This server is invite-only. Ask an admin to invite you.'
	};

	let email = $state('');
	let password = $state('');
	const initialError = page.url.searchParams.get('error');

	let error = $state(
		initialError ? (ERROR_MESSAGES[initialError] ?? 'Sign-in failed. Please try again.') : ''
	);
	let loading = $state(false);
	let passwordLoginEnabled = $state(true);
	let providersLoaded = $state(false);

	if (initialError) {
		const cleanUrl = new URL(window.location.href);
		cleanUrl.searchParams.delete('error');
		window.history.replaceState({}, '', cleanUrl.pathname + cleanUrl.search);
	}

	// Get returnTo parameter for redirecting after login
	const returnTo = $derived(page.url.searchParams.get('returnTo'));

	if (!__CLOUD_MODE__) {
		$effect(() => {
			// Wait for providers to load - if password login is disabled, skip the /register redirect.
			if (!providersLoaded) return;
			if (!passwordLoginEnabled) return;

			loading = true;
			fetch('/api/has-organizations', {
				method: 'GET'
			})
				.then((response) => response.json())
				.then((response) => {
					if (response.hasOrganizations) {
						loading = false;
					} else {
						goto(resolve('/register'));
					}
				})
				.catch(() => {
					toast.error('An unexpected error has occurred. The page will refresh in 5 seconds.');
					setTimeout(() => {
						window.location.reload();
					}, 5000);
				});
		});
	}

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

			authState.setToken(data.token);
			authState.setOrganizations(data.organizations || []);
			projectsState.setProjects(data.projects);

			// Redirect to returnTo if provided (validated same-origin), otherwise dashboard
			gotoHref(safeLocalPath(returnTo));
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
				<ErrorAlert {error} class="mb-4" />
			{/if}
			<OauthButtons bind:passwordLoginEnabled bind:loaded={providersLoaded} />
			{#if providersLoaded && passwordLoginEnabled}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleLogin();
					}}
					class="grid w-full items-center gap-4"
				>
					<div class="flex flex-col space-y-1.5">
						<Label for="email">Email</Label>
						<Input
							id="email"
							type="email"
							bind:value={email}
							placeholder="you@example.com"
							required
						/>
					</div>
					<div class="flex flex-col space-y-1.5">
						<Label for="password">Password</Label>
						<Input
							id="password"
							type="password"
							bind:value={password}
							placeholder="Password"
							required
						/>
						<a
							href={resolve('/forgot-password')}
							class="self-end text-sm text-primary hover:underline">Forgot password?</a
						>
					</div>
					<Button type="submit" disabled={loading} class="w-full">
						{#if loading}
							Logging in...
						{:else}
							Login
						{/if}
					</Button>
				</form>
			{/if}
		</CardContent>

		<!-- If the backend is running in the cloud mode we'll allow registration to take place -->
		<!-- If we're running in the self hosted mode - we will not allow register -->
		{#if __CLOUD_MODE__}
			<CardFooter class="flex flex-col justify-center">
				<p class="text-sm text-muted-foreground">
					Don't have an account? <a href={resolve('/register')} class="text-primary hover:underline"
						>Create account</a
					>
				</p>
			</CardFooter>
		{/if}
	</Card>
</div>
