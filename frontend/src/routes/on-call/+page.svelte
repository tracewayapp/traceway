<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { oncallState } from '$lib/state/oncall.svelte';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import OverviewTab from './overview-tab.svelte';
	import TeamsTab from './teams-tab.svelte';
	import SchedulesTab from './schedules-tab.svelte';
	import PagesTab from './pages-tab.svelte';
	import PoliciesTab from './policies-tab.svelte';

	const orgs = $derived(authState.organizations);

	let selectedOrgId = $state<number | null>(null);

	const initialPageParam = page.url.searchParams.get('page');
	const deepLinkPageId =
		initialPageParam && !Number.isNaN(Number(initialPageParam)) ? Number(initialPageParam) : null;

	// Ack links carry the page's project id: pages resolve against the selected
	// project, so switch before the pages tab issues its first request.
	const initialProjectParam = page.url.searchParams.get('projectId');
	if (initialProjectParam && initialProjectParam !== projectsState.currentProjectId) {
		projectsState.selectProject(initialProjectParam);
	}

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

	const canManage = $derived(
		currentOrganizationId !== null && oncallState.canManage(currentOrganizationId)
	);

	const activeTab = $derived(page.url.searchParams.get('tab') || 'pages');

	function setTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	$effect(() => {
		if (currentOrganizationId !== null) {
			oncallState.loadTeams(currentOrganizationId);
			oncallState.loadSchedules(currentOrganizationId);
		}
	});

	onMount(() => {
		if (deepLinkPageId !== null) {
			const url = new URL(window.location.href);
			url.searchParams.set('tab', 'pages');
			url.searchParams.delete('page');
			url.searchParams.delete('projectId');
			goto(url.toString(), { replaceState: true, noScroll: true });
		}
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between gap-4">
		<h1 class="text-3xl font-semibold tracking-tight">On-Call</h1>
		{#if orgs.length > 1}
			<div class="flex items-center gap-2">
				<span class="text-sm text-muted-foreground">Organization</span>
				<Select.Root
					type="single"
					value={currentOrganizationId !== null ? String(currentOrganizationId) : undefined}
					onValueChange={(val) => {
						if (val) selectedOrgId = Number(val);
					}}
				>
					<Select.Trigger class="w-[220px]">
						{currentOrganizationName || 'Select organization'}
					</Select.Trigger>
					<Select.Content>
						{#each orgs as org (org.id)}
							<Select.Item value={String(org.id)}>{org.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
		{/if}
	</div>

	<Tabs.Root
		value={activeTab}
		onValueChange={(v) => {
			if (v) setTab(v);
		}}
	>
		<Tabs.List>
			<Tabs.Trigger value="pages">Pages</Tabs.Trigger>
			<Tabs.Trigger value="overview">Overview</Tabs.Trigger>
			<Tabs.Trigger value="teams">Teams</Tabs.Trigger>
			<Tabs.Trigger value="schedules">Schedules</Tabs.Trigger>
			<Tabs.Trigger value="policies">Policies</Tabs.Trigger>
		</Tabs.List>
	</Tabs.Root>

	{#if currentOrganizationId === null}
		<div
			class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
		>
			<p>You are not a member of any organization yet.</p>
		</div>
	{:else if activeTab === 'pages'}
		<PagesTab {deepLinkPageId} />
	{:else if activeTab === 'overview'}
		<OverviewTab organizationId={currentOrganizationId} onGoToTeams={() => setTab('teams')} />
	{:else if activeTab === 'teams'}
		<TeamsTab organizationId={currentOrganizationId} {canManage} />
	{:else if activeTab === 'schedules'}
		<SchedulesTab organizationId={currentOrganizationId} {canManage} />
	{:else if activeTab === 'policies'}
		<PoliciesTab organizationId={currentOrganizationId} {canManage} />
	{/if}
</div>
