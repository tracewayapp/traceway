<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { ArrowLeft, Check, LogOut, Plus } from '@lucide/svelte';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { toast } from 'svelte-sonner';
	import { authState, type UserOrganizationResponse } from '$lib/state/auth.svelte';
	import { projectsState, type Project } from '$lib/state/projects.svelte';
	import { api } from '$lib/api';
	import { getErrorMessage } from '$lib/utils/errors';
	import SetupProjectsStep from '$lib/components/setup/setup-projects-step.svelte';

	let { data } = $props();

	const writableOrgs = $derived(authState.organizations.filter((o) => o.role !== 'readonly'));

	let selectedOrgId = $state<number | null>(null);

	$effect(() => {
		if (selectedOrgId !== null && writableOrgs.some((o) => o.id === selectedOrgId)) return;
		const preferred = [data.organizationId, projectsState.currentProject?.organizationId].find(
			(id) => id != null && writableOrgs.some((o) => o.id === id)
		);
		selectedOrgId = preferred ?? writableOrgs[0]?.id ?? null;
	});

	const selectedOrgName = $derived(writableOrgs.find((o) => o.id === selectedOrgId)?.name ?? '');

	// The cached organization list can be stale after a mid-session removal.
	let created = false;
	onMount(async () => {
		try {
			const bundle = (await api.get('/me/login-bundle')) as {
				organizations?: UserOrganizationResponse[];
				projects?: Project[];
			};
			if (created) return;
			authState.setOrganizations(bundle.organizations || []);
			projectsState.setProjects(bundle.projects || []);
		} catch {
			// keep the cached list
		}
	});

	const hasNoOrganizations = $derived(authState.organizations.length === 0);

	let step = $state<'choice' | 'organization'>('choice');
	let newOrgName = $state('');
	let timezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone);
	let creating = $state(false);
	let createError = $state('');

	const timezones = Intl.supportedValuesOf('timeZone');

	async function createOrganization(event: SubmitEvent) {
		event.preventDefault();
		creating = true;
		createError = '';
		try {
			const organization = (await api.post('/organizations', {
				name: newOrgName,
				timezone
			})) as UserOrganizationResponse;
			created = true;
			authState.setOrganizations([...authState.organizations, organization]);
			newOrgName = '';
			toast.success('Successfully created the Organization', { position: 'top-center' });
		} catch (e) {
			createError = getErrorMessage(e, 'Failed to create the organization');
		} finally {
			creating = false;
		}
	}

	function handleLogout() {
		authState.logout();
		projectsState.clear();
		goto(resolve('/login'));
	}
</script>

<div class="mx-auto w-full max-w-2xl space-y-6">
	{#if hasNoOrganizations}
		{#if step === 'choice'}
			<div>
				<h1 class="text-2xl font-bold">You're not in an organization</h1>
				<p class="mt-1 text-sm text-muted-foreground">
					Looks like you are not part of any organizations. Would you like to register for one? If
					you were removed from one, an admin can also invite you back — logging out and returning
					once the invitation arrives keeps you in that organization rather than starting a second.
				</p>
			</div>

			<div class="flex flex-wrap gap-2">
				<Button variant="success" onclick={() => (step = 'organization')}>
					<Plus class="mr-2 size-4" />
					New Organization
				</Button>
				<Button variant="outline" onclick={handleLogout}>
					<LogOut class="mr-2 size-4" />
					Log out
				</Button>
			</div>
		{:else}
			<div>
				<h1 class="text-2xl font-bold">Create an Organization</h1>
				<p class="mt-1 text-sm text-muted-foreground">
					Name it and pick the timezone its on-call schedules should follow. You'll own it, and can
					invite the rest of your team afterwards.
				</p>
			</div>

			<form class="space-y-4" onsubmit={createOrganization}>
				<div class="flex max-w-sm flex-col space-y-1.5">
					<Label for="organization-name">Organization name</Label>
					<Input
						id="organization-name"
						bind:value={newOrgName}
						placeholder="Acme Inc."
						autocomplete="organization"
					/>
				</div>

				<div class="flex max-w-sm flex-col space-y-1.5">
					<Label for="timezone">Timezone</Label>
					<Select.Root type="single" bind:value={timezone}>
						<Select.Trigger id="timezone" class="w-full">
							<span>{timezone}</span>
						</Select.Trigger>
						<Select.Content class="max-h-60">
							{#each timezones as tz (tz)}
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

				<ErrorAlert error={createError} />

				<div class="flex flex-wrap gap-2">
					<Button type="submit" variant="success" disabled={creating}>
						<Plus class="mr-2 size-4" />
						New Organization
					</Button>
					<Button
						type="button"
						variant="ghost"
						disabled={creating}
						onclick={() => (step = 'choice')}
					>
						Back
					</Button>
				</div>
			</form>
		{/if}
	{:else}
		<div class="flex items-start justify-between gap-4">
			<div class="min-w-0">
				<h1 class="text-2xl font-bold">
					{data.existingAccount ? 'Add Projects' : 'Set Up Projects'}
				</h1>
				<p class="mt-1 text-sm text-muted-foreground">
					Let your coding agent propose the project setup for your approval, or create projects
					manually.
				</p>
			</div>
			{#if data.existingAccount}
				<Button variant="outline" size="sm" class="shrink-0" onclick={() => goto(resolve('/'))}>
					<ArrowLeft class="mr-2 size-4" />
					Back to Dashboard
				</Button>
			{/if}
		</div>

		{#if writableOrgs.length === 0}
			<p class="text-sm text-muted-foreground">
				You need an owner, admin, or user role in an organization to create projects.
			</p>
		{:else}
			{#if writableOrgs.length > 1}
				<div class="flex max-w-xs flex-col space-y-1.5">
					<Label>Organization</Label>
					<Select.Root
						type="single"
						value={selectedOrgId !== null ? String(selectedOrgId) : undefined}
						onValueChange={(val) => {
							if (val) selectedOrgId = Number(val);
						}}
					>
						<Select.Trigger class="w-full">
							{selectedOrgName || 'Select organization'}
						</Select.Trigger>
						<Select.Content>
							{#each writableOrgs as org (org.id)}
								<Select.Item value={String(org.id)}>{org.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			{/if}

			{#if selectedOrgId !== null}
				{#key selectedOrgId}
					<SetupProjectsStep
						organizationId={selectedOrgId}
						initialMode={data.existingAccount ? 'ai' : undefined}
					/>
				{/key}
			{/if}
		{/if}
	{/if}
</div>
