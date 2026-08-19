<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		Card,
		CardContent,
		CardFooter,
		CardHeader,
		CardTitle,
		CardDescription
	} from '$lib/components/ui/card';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Select from '$lib/components/ui/select';
	import { Check } from '@lucide/svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState, FRAMEWORK_LABELS, type Framework } from '$lib/state/projects.svelte';
	import { themeState } from '$lib/state/theme.svelte';
	import { toast } from 'svelte-sonner';
	import TurnstileWidget from '$lib/components/turnstile-widget.svelte';
	import OauthButtons from '$lib/components/oauth-buttons.svelte';
	import SetupProjectsStep from '$lib/components/setup/setup-projects-step.svelte';

	const DEFAULT_FRAMEWORK: Framework = 'opentelemetry';

	function parseFrameworkParam(value: string | null): Framework {
		if (!value) return DEFAULT_FRAMEWORK;
		return (value in FRAMEWORK_LABELS ? value : DEFAULT_FRAMEWORK) as Framework;
	}

	let step = $state<'account' | 'projects'>('account');
	let email = $state(page.url.searchParams.get('email') ?? '');
	let name = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let organizationName = $state('');
	let timezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone);
	let error = $state('');
	let loading = $state(false);
	let captchaToken = $state('');
	let passwordLoginEnabled = $state(true);
	let providersLoaded = $state(false);
	let newOrgId = $state<number | null>(null);
	let turnstileWidget = $state<TurnstileWidget | null>(null);

	const initialFramework = parseFrameworkParam(page.url.searchParams.get('framework'));

	const turnstileSiteKey = __TURNSTILE_SITE_KEY__;
	const captchaEnabled = turnstileSiteKey !== '';

	const timezones = Intl.supportedValuesOf('timeZone');

	if (!__CLOUD_MODE__) {
		$effect(() => {
			if (!providersLoaded || step !== 'account') return;

			// Password login is disabled - this page is inaccessible, send to login.
			if (!passwordLoginEnabled) {
				goto(resolve('/login'));
				return;
			}

			// if we're not in the cloud mode we have to check if an organization exists and if it does we should go to the login page
			loading = true;
			fetch('/api/has-organizations', {
				method: 'GET'
			})
				.then((response) => response.json())
				.then((response) => {
					if (response.hasOrganizations) {
						goto(resolve('/login'));
					}
					loading = false;
				})
				.catch(() => {
					toast.error('An unexpected error has occurred. The page will refresh in 5 seconds.');
					setTimeout(() => {
						window.location.reload();
					}, 5000);
				});
		});
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
			// Writing the (empty) projects cache here is load-bearing: a
			// refresh mid-wizard routes authenticated zero-project users to
			// /setup based on it.
			projectsState.setProjects(data.projects ?? []);
			newOrgId = data.organizations?.[0]?.id ?? null;

			if (newOrgId === null) {
				goto(resolve('/'));
				return;
			}
			step = 'projects';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Registration failed';
			turnstileWidget?.reset();
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen w-full items-center justify-center px-4 py-8">
	<Card class={step === 'projects' ? 'w-full max-w-2xl' : 'w-[400px]'}>
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
				{#if step === 'projects'}
					Set up your projects
				{:else}
					Create your account
				{/if}
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if step === 'projects' && newOrgId !== null}
				<SetupProjectsStep
					organizationId={newOrgId}
					{initialFramework}
					onDone={() => goto(resolve('/'))}
					onSkip={() => goto(resolve('/'))}
					continueLabel="Continue to Dashboard"
				/>
			{:else}
				{#if error}
					<ErrorAlert {error} class="mb-4" />
				{/if}
				<OauthButtons bind:passwordLoginEnabled bind:loaded={providersLoaded} />
				{#if providersLoaded && passwordLoginEnabled}
					<form
						onsubmit={(e) => {
							e.preventDefault();
							handleRegister();
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
							<Label for="name">Name</Label>
							<Input id="name" type="text" bind:value={name} placeholder="Your name" required />
						</div>
						<div class="flex flex-col space-y-1.5">
							<Label for="password">Password</Label>
							<Input
								id="password"
								type="password"
								bind:value={password}
								placeholder="Password (min 8 characters)"
								required
							/>
						</div>
						<div class="flex flex-col space-y-1.5">
							<Label for="confirmPassword">Confirm Password</Label>
							<Input
								id="confirmPassword"
								type="password"
								bind:value={confirmPassword}
								placeholder="Confirm password"
								required
							/>
						</div>

						<div class="mt-2 flex items-center gap-3">
							<div class="flex-1 border-t"></div>
							<p class="text-sm text-muted-foreground">Organization</p>
							<div class="flex-1 border-t"></div>
						</div>

						<div class="flex flex-col space-y-1.5">
							<Label for="organizationName">Organization Name</Label>
							<Input
								id="organizationName"
								type="text"
								bind:value={organizationName}
								placeholder="Your company or team"
								required
							/>
						</div>
						<div class="flex flex-col space-y-1.5">
							<Label for="timezone">Timezone</Label>
							<Select.Root type="single" bind:value={timezone}>
								<Select.Trigger class="w-full">
									<span>{timezone}</span>
								</Select.Trigger>
								<Select.Content class="max-h-60">
									{#each timezones as tz, __index (__index)}
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

						{#if captchaEnabled}
							<div class="mt-2 flex flex-col space-y-1.5">
								<TurnstileWidget
									bind:this={turnstileWidget}
									siteKey={turnstileSiteKey}
									onVerify={(token) => (captchaToken = token)}
									onError={() => (captchaToken = '')}
								/>
							</div>
						{/if}

						<Button
							type="submit"
							disabled={loading || (captchaEnabled && !captchaToken)}
							class="mt-2 w-full"
						>
							{#if loading}
								Creating account...
							{:else}
								Create Account
							{/if}
						</Button>
					</form>
				{/if}
			{/if}
		</CardContent>

		{#if __CLOUD_MODE__ && step === 'account'}
			<CardFooter class="flex flex-col justify-center">
				<p class="text-sm text-muted-foreground">
					Already have an account? <a href={resolve('/login')} class="text-primary hover:underline"
						>Login</a
					>
				</p>
			</CardFooter>
		{/if}
	</Card>
</div>
