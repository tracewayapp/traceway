<script lang="ts">
	import { page } from '$app/state';
	import { authState } from '$lib/state/auth.svelte';
	import { setTabParam } from '$lib/utils/url-params';
	import { gotoHref } from '$lib/utils/navigation';
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import OverviewTab from './overview-tab.svelte';
	import IssuesTab from './issues-tab.svelte';
	import MonitorsTab from './monitors-tab.svelte';
	import ProjectsTab from './projects-tab.svelte';

	const TABS = [
		{ value: 'overview', label: 'Overview' },
		{ value: 'issues', label: 'Issues' },
		{ value: 'monitors', label: 'Monitors' },
		{ value: 'projects', label: 'Projects' }
	];

	const TAB_DESCRIPTIONS: Record<string, string> = {
		issues: 'Recently active issues from every project, ordered by the last event received.',
		monitors:
			'All monitors from every project in the organization in one list, with their current status, uptime, and recent incidents.',
		projects:
			'Every project in the organization, with its framework and your effective access level.'
	};

	const orgs = $derived(authState.organizations);

	const currentOrganizationId = $derived.by(() => {
		const value = Number(page.url.searchParams.get('organizationId'));
		if (Number.isInteger(value) && orgs.some((organization) => organization.id === value)) {
			return value;
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

	$effect(() => {
		const organizationId = currentOrganizationId;
		if (organizationId === null) return;
		if (
			page.url.searchParams.get('organizationId') === String(organizationId) &&
			!page.url.searchParams.has('projectId')
		) {
			return;
		}
		const url = new URL(page.url);
		url.searchParams.set('organizationId', String(organizationId));
		url.searchParams.delete('projectId');
		gotoHref(url.pathname + url.search, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	});
</script>

<div class="space-y-4">
	<PageHeader
		title={currentOrganizationName || 'Organization'}
		description="Live instance health, active response, and telemetry across every project."
	/>

	<PageTabs tabs={TABS} {activeTab} onTabChange={setTab} />

	{#if TAB_DESCRIPTIONS[activeTab]}
		<InfoCallout>{TAB_DESCRIPTIONS[activeTab]}</InfoCallout>
	{/if}

	{#if currentOrganizationId === null}
		<EmptyState message="You are not a member of any organization yet." />
	{:else if activeTab === 'overview'}
		<OverviewTab organizationId={currentOrganizationId} />
	{:else if activeTab === 'issues'}
		<IssuesTab organizationId={currentOrganizationId} />
	{:else if activeTab === 'monitors'}
		<MonitorsTab organizationId={currentOrganizationId} />
	{:else if activeTab === 'projects'}
		<ProjectsTab organizationId={currentOrganizationId} />
	{/if}
</div>
