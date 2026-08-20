<script lang="ts">
	import { page } from '$app/state';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { setTabParam } from '$lib/utils/url-params';
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import OrgSwitcher from '$lib/components/traceway/org-switcher.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import OverviewTab from './overview-tab.svelte';
	import MonitorsTab from './monitors-tab.svelte';

	const TABS = [
		{ value: 'overview', label: 'Overview' },
		{ value: 'monitors', label: 'Monitors' }
	];

	const TAB_DESCRIPTIONS: Record<string, string> = {
		overview:
			'A single place to see what needs attention across every project in the organization: servers reporting in, recent issues, and open on-call pages.',
		monitors:
			'All monitors from every project in the organization in one list, with their current status, uptime, and recent incidents.'
	};

	const orgs = $derived(authState.organizations);

	let selectedOrgId = $state<number | null>(null);

	const currentOrganizationId = $derived.by(() => {
		if (selectedOrgId !== null && orgs.some((o) => o.id === selectedOrgId)) {
			return selectedOrgId;
		}
		const projectOrgId = projectsState.currentProject?.organizationId;
		if (projectOrgId && orgs.some((o) => o.id === projectOrgId)) {
			return projectOrgId;
		}
		return orgs[0]?.id ?? null;
	});

	const currentOrganizationName = $derived(
		orgs.find((o) => o.id === currentOrganizationId)?.name ?? ''
	);

	const activeTab = $derived.by(() => {
		const tab = page.url.searchParams.get('tab') || 'overview';
		return TABS.some((t) => t.value === tab) ? tab : 'overview';
	});

	function setTab(tab: string) {
		setTabParam(tab);
	}
</script>

<div class="space-y-4">
	<PageHeader title="Organization" subtitle={orgs.length > 1 ? undefined : currentOrganizationName}>
		{#snippet actions()}
			{#if orgs.length > 1}
				<OrgSwitcher
					organizations={orgs.map((o) => ({ id: o.id, name: o.name }))}
					{currentOrganizationId}
					{currentOrganizationName}
					onChange={(id) => (selectedOrgId = id)}
				/>
			{/if}
		{/snippet}
	</PageHeader>

	<PageTabs tabs={TABS} {activeTab} onTabChange={setTab} />

	{#if TAB_DESCRIPTIONS[activeTab]}
		<InfoCallout>{TAB_DESCRIPTIONS[activeTab]}</InfoCallout>
	{/if}

	{#if currentOrganizationId === null}
		<EmptyState message="You are not a member of any organization yet." />
	{:else if activeTab === 'overview'}
		<OverviewTab organizationId={currentOrganizationId} />
	{:else if activeTab === 'monitors'}
		<MonitorsTab organizationId={currentOrganizationId} />
	{/if}
</div>
