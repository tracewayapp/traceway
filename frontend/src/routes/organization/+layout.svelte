<script lang="ts">
	import { page } from '$app/state';
	import { projectsState } from '$lib/state/projects.svelte';
	import { organizationContext, organizationHref } from '$lib/state/organization-context.svelte';
	import { gotoHref } from '$lib/utils/navigation';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';

	let { children } = $props();

	const LEGACY_TABS: Record<string, string> = {
		overview: '/organization',
		issues: '/organization/issues',
		monitors: '/organization/monitors',
		projects: '/organization/projects'
	};

	const BACKEND_ONLY_PATHS = new Set(['/organization', '/organization/monitors']);

	const redirectHref = $derived.by(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null || !projectsState.loaded) return null;
		const projects = organizationContext.projects;
		if (projects.length === 1) return `/?projectId=${projects[0].id}`;
		const tab = page.url.searchParams.get('tab');
		if (tab && LEGACY_TABS[tab]) return organizationHref(LEGACY_TABS[tab], organizationId);
		if (
			projects.length > 0 &&
			!organizationContext.hasBackendProjects &&
			BACKEND_ONLY_PATHS.has(page.url.pathname)
		) {
			return organizationHref('/organization/issues', organizationId);
		}
		return null;
	});

	$effect(() => {
		const organizationId = organizationContext.organizationId;
		if (organizationId === null || !projectsState.loaded) return;
		if (redirectHref) {
			gotoHref(redirectHref, { replaceState: true });
			return;
		}
		const params = page.url.searchParams;
		if (
			params.get('organizationId') === String(organizationId) &&
			!params.has('projectId') &&
			!params.has('tab')
		) {
			return;
		}
		const url = new URL(page.url);
		url.searchParams.set('organizationId', String(organizationId));
		url.searchParams.delete('projectId');
		url.searchParams.delete('tab');
		gotoHref(url.pathname + url.search, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	});
</script>

{#if !projectsState.loaded || redirectHref}
	<div class="flex h-48 items-center justify-center">
		<LoadingCircle size="xlg" />
	</div>
{:else if organizationContext.organizationId === null}
	<EmptyState message="You are not a member of any organization yet." />
{:else if organizationContext.projects.length === 0}
	<EmptyState message="This organization has no projects yet." />
{:else}
	{@render children()}
{/if}
