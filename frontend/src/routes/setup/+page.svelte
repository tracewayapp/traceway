<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Check, LogOut, Plus } from '@lucide/svelte';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { toast } from 'svelte-sonner';
	import { authState, type UserOrganizationResponse } from '$lib/state/auth.svelte';
	import { projectsState, type Project } from '$lib/state/projects.svelte';
	import { api } from '$lib/api';
	import SetupProjectsStep from '$lib/components/setup/setup-projects-step.svelte';

	const writableOrgs = $derived(authState.organizations.filter((o) => o.role !== 'readonly'));

	let selectedOrgId = $state<number | null>(null);

	$effect(() => {
		if (selectedOrgId !== null && writableOrgs.some((o) => o.id === selectedOrgId)) return;
		const currentOrgId = projectsState.currentProject?.organizationId;
		selectedOrgId = writableOrgs.some((o) => o.id === currentOrgId)
			? currentOrgId!
			: (writableOrgs[0]?.id ?? null);
	});

	const selectedOrgName = $derived(writableOrgs.find((o) => o.id === selectedOrgId)?.name ?? '');

	// authState.organizations is hydrated from localStorage and only rewritten on
	// login, so a membership removed mid-session is still cached here. The layout
	// pins every zero-project account to this page, which makes it the one screen
	// that must not act on a stale list.
	// Deliberately not a render gate: the cached list is right for every case but
	// the mid-session removal this endpoint exists for, so blocking the page would
	// tax every onboarding load to avoid one rare branch flip.

	onMount(async () => {
		try {
			const bundle = (await api.get('/me/login-bundle')) as {
				organizations?: UserOrganizationResponse[];
				projects?: Project[];
			};
			authState.setOrganizations(bundle.organizations || []);
			projectsState.setProjects(bundle.projects || []);
		} catch {
			// Fall back to the cached list rather than stranding the page; a genuinely
			// expired token is already handled by the 401 path in api.ts.
		}
	});

	const hasNoOrganizations = $derived(authState.organizations.length === 0);

	// A user with no organization is offered a choice before a form, because
	// creating one is not always the right answer: on a self-hosted instance that
	// already has an organization the endpoint refuses (422, "ask an administrator
	// to invite you"), and being removed by an admin is usually followed by an
	// invitation rather than by starting a rival organization. So the alternative
	// -- log out and come back when the invite lands -- is a first-class action
	// here, not something to hunt for in a nav bar this page does not render.
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
			// Appending flips hasNoOrganizations, which swaps this page over to its
			// project-setup half -- the /setup the user is meant to land on. A goto()
			// to the route they are already on would remount for no gain and refetch
			// the login bundle the response just superseded.
			authState.setOrganizations([...authState.organizations, organization]);

			newOrgName = '';
			toast.success('Successfully created the Organization', { position: 'top-center' });
		} catch (e) {
			createError =
				e instanceof Error && e.message ? e.message : 'Failed to create the organization';
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
		<div>
			<h1 class="text-2xl font-bold">Set Up Projects</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Let your coding agent propose the project setup for your approval, or create projects
				manually.
			</p>
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
					<SetupProjectsStep organizationId={selectedOrgId} />
				{/key}
			{/if}
		{/if}
	{/if}
</div>
