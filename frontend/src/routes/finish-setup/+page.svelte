<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Select from '$lib/components/ui/select';
	import { Check } from '@lucide/svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { themeState } from '$lib/state/theme.svelte';
	import { consumeSsoReturnTo, safeLocalPath } from '$lib/utils/navigation';
	import SetupProjectsStep from '$lib/components/setup/setup-projects-step.svelte';

	let phase = $state<'organization' | 'projects'>('organization');
	let organizationName = $state('');
	let timezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone);
	let error = $state('');
	let loading = $state(false);
	let newOrgId = $state<number | null>(null);
	let returnTo = $state('/');

	const timezones = Intl.supportedValuesOf('timeZone');

	onMount(() => {
		if (!authState.token) {
			goto(resolve('/login'));
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
					Authorization: `Bearer ${authState.token}`
				},
				body: JSON.stringify({ organizationName, timezone })
			});

			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				throw new Error(data.error || 'Setup failed');
			}

			const data = await response.json();
			authState.setToken(data.token);
			authState.setOrganizations(data.organizations || []);
			projectsState.setProjects(data.projects ?? []);
			newOrgId = data.organizations?.[0]?.id ?? null;
			// The stashed SSO returnTo is single-use; capture it now and use
			// it when the project step finishes.
			returnTo = safeLocalPath(consumeSsoReturnTo());

			if (newOrgId === null) {
				goto(returnTo);
				return;
			}
			phase = 'projects';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Setup failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen w-full items-center justify-center px-4 py-8">
	<Card class={phase === 'projects' ? 'w-full max-w-2xl' : 'w-[400px]'}>
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
				{#if phase === 'projects'}
					Set up your projects
				{:else}
					Finish setting up your account
				{/if}
			</CardDescription>
		</CardHeader>
		<CardContent>
			{#if phase === 'projects' && newOrgId !== null}
				<SetupProjectsStep
					organizationId={newOrgId}
					onDone={() => goto(returnTo)}
					onSkip={() => goto(returnTo)}
					continueLabel="Continue"
				/>
			{:else}
				{#if error}
					<ErrorAlert {error} class="mb-4" />
				{/if}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleSubmit();
					}}
					class="grid w-full items-center gap-4"
				>
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
					<Button type="submit" disabled={loading} class="mt-2 w-full">
						{#if loading}
							Finishing setup...
						{:else}
							Finish setup
						{/if}
					</Button>
				</form>
			{/if}
		</CardContent>
	</Card>
</div>
