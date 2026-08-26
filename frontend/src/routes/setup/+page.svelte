<script lang="ts">
	import { onMount } from 'svelte';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Plus } from '@lucide/svelte';
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

	let newOrgName = $state('');
	let creating = $state(false);
	let createError = $state('');

	async function createOrganization(event: SubmitEvent) {
		event.preventDefault();
		creating = true;
		createError = '';
		try {
			const organization = (await api.post('/organizations', {
				name: newOrgName,
				timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
			})) as UserOrganizationResponse;
			authState.setOrganizations([...authState.organizations, organization]);

			newOrgName = '';
			toast.success('Successfully created the Organization', { position: 'top-center' });
		} catch (e: any) {
			createError = e.message || 'Failed to create the organization';
			creating = false;
		}
	}
</script>

<div class="mx-auto w-full max-w-2xl space-y-6">
	{#if hasNoOrganizations}
		<div>
			<h1 class="text-2xl font-bold">Create an Organization</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				You're not a member of any organization right now — if you were removed from one, an admin
				can invite you back. You can also start your own.
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

			<ErrorAlert error={createError} />

			<Button type="submit" variant="success" disabled={creating}>
				<Plus class="mr-2 size-4" />
				New Organization
			</Button>
		</form>
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
