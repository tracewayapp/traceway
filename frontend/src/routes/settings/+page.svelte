<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { organizationState } from '$lib/state/organization.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import OrgSwitcher from '$lib/components/traceway/org-switcher.svelte';
	import OrganizationTab from './organization-tab.svelte';
	import UsersTab from './users-tab.svelte';
	import type { Component } from 'svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let BillingTab = $state<Component<{ organizationId: number }> | null>(null);

	async function loadBillingModule() {
		try {
			// @ts-expect-error - $billing alias only exists when billing extension is available
			const module = await import('$billing/billing-tab.svelte');
			BillingTab = module.default;
		} catch {
			// Billing extension not available - this is expected for open source builds
		}
	}

	$effect(() => {
		loadBillingModule();
	});

	const manageableOrgs = $derived(
		authState.organizations.filter((o) => o.role === 'owner' || o.role === 'admin')
	);

	let selectedOrgId = $state<number | null>(null);

	const currentOrganizationId = $derived.by(() => {
		if (selectedOrgId !== null && manageableOrgs.some((o) => o.id === selectedOrgId)) {
			return selectedOrgId;
		}
		const projectOrgId = projectsState.currentProject?.organizationId;
		if (projectOrgId && manageableOrgs.some((o) => o.id === projectOrgId)) {
			return projectOrgId;
		}
		return manageableOrgs[0]?.id ?? null;
	});

	const currentOrganizationName = $derived(
		manageableOrgs.find((o) => o.id === currentOrganizationId)?.name ?? ''
	);

	const hasAccess = $derived(currentOrganizationId !== null);

	$effect(() => {
		if (!hasAccess) {
			goto(resolve('/'));
		}
	});

	let loadSeq = 0;

	$effect(() => {
		if (currentOrganizationId && hasAccess) {
			const seq = ++loadSeq;
			loading = true;
			error = null;
			organizationState.loadSettings(currentOrganizationId).then(() => {
				if (seq !== loadSeq) return;
				error = organizationState.error;
				loading = false;
			});
		}
	});
</script>

<div class="space-y-4">
	<PageHeader title="Settings">
		{#snippet actions()}
			{#if manageableOrgs.length > 1}
				<OrgSwitcher
					organizations={manageableOrgs.map((o) => ({ id: o.id, name: o.name }))}
					{currentOrganizationId}
					{currentOrganizationName}
					onChange={(id) => (selectedOrgId = id)}
				/>
			{/if}
		{/snippet}
	</PageHeader>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<LoadingCircle size="xlg" />
		</div>
	{:else if error}
		<div class="py-12 text-center text-destructive">
			{error}
		</div>
	{:else}
		<div class="space-y-4">
			<OrganizationTab />
			<UsersTab organizationId={currentOrganizationId!} />
			{#if BillingTab}
				<BillingTab organizationId={currentOrganizationId!} />
			{/if}
		</div>
	{/if}
</div>
