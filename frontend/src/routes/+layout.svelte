<script lang="ts">
	import { resolve } from '$app/paths';
	import './layout.css';
	import { goto, afterNavigate } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState, type Project } from '$lib/state/projects.svelte';
	import { themeState, initTheme, toggleTheme } from '$lib/state/theme.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { DateTime } from 'luxon';
	import { incrementNavDepth, decrementNavDepth, clearNavDepth } from '$lib/utils/back-navigation';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import AddProjectModal from '$lib/components/add-project-modal.svelte';
	import EditProjectModal from '$lib/components/edit-project-modal.svelte';
	import DashboardCommand from '$lib/components/dashboard/dashboard-command.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Button } from '$lib/components/ui/button';
	import { Sun, Moon, LogOut, Plus, Check, Pencil } from '@lucide/svelte';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import { onMount } from 'svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { ChevronDown } from '@lucide/svelte';
	import { Toaster, toast } from 'svelte-sonner';
	import { page } from '$app/state';
	import { setupTraceway } from '@tracewayapp/svelte';
	import { captureException } from '@tracewayapp/frontend';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import type { Component } from 'svelte';

	if (__TRACEWAY_URL__) {
		setupTraceway({
			connectionString: __TRACEWAY_URL__
		});
	}

	let { children } = $props();
	let showAddProjectModal = $state(false);
	let showEditProjectModal = $state(false);
	let showCommandPalette = $state(false);

	const isMacPlatform =
		typeof navigator !== 'undefined' && /Mac|iP(hone|ad|od)/.test(navigator.platform);

	function handleGlobalKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			if (!authState.isAuthenticated || isPublicPath(page.url.pathname)) return;
			if (isMacPlatform && !e.metaKey) return;
			if (page.url.pathname === '/dashboards') return;
			e.preventDefault();
			showCommandPalette = !showCommandPalette;
		}
	}

	const SIDEBAR_OPEN_KEY = 'traceway_sidebar_open';
	let sidebarOpen = $state(localStorage.getItem(SIDEBAR_OPEN_KEY) !== 'false');
	let CrossSiteNotificationBanner = $state<Component<{ organizationId: number }> | null>(null);

	const bannerOrganizationId = $derived(projectsState.currentProject?.organizationId ?? null);

	// Warn when the browser's timezone renders times differently from the
	// organization's timezone (all timestamps are shown in the org timezone).
	// Compare offsets, not zone names — Europe/Berlin vs Europe/Belgrade is
	// not a mismatch worth warning about.
	const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	const orgZone = $derived(getTimezone());
	const timezoneMismatch = $derived.by(() => {
		if (!orgZone || orgZone === browserZone) return false;
		const now = Date.now();
		return (
			DateTime.fromMillis(now, { zone: orgZone }).offset !==
			DateTime.fromMillis(now, { zone: browserZone }).offset
		);
	});

	function zoneLabel(zone: string): string {
		if (zone === 'UTC') return 'UTC';
		return `${zone}, UTC${DateTime.now().setZone(zone).toFormat('Z')}`;
	}

	const PUBLIC_PATHS = new Set([
		'/login',
		'/register',
		'/auth/callback',
		'/finish-setup',
		'/forgot-password',
		'/reset-password',
		'/device',
		'/oauth/authorize'
	]);

	function isPublicPath(pathname: string): boolean {
		return (
			PUBLIC_PATHS.has(pathname) ||
			pathname.startsWith('/accept-invitation') ||
			pathname.startsWith('/ack/') ||
			pathname.startsWith('/status/')
		);
	}

	// An unauthenticated visit to a protected path matches neither layout
	// branch below and would render a blank page; send it to login instead,
	// carrying the original URL so login can return there.
	$effect(() => {
		if (!authState.isAuthenticated && !isPublicPath(page.url.pathname)) {
			const returnTo = page.url.pathname + page.url.search;
			if (page.url.pathname === '/') {
				// A CNAMEd status-page vanity domain lands here anonymously; ask
				// the backend whether this Host maps to a status page before
				// falling back to the login redirect.
				fetch('/api/status-domains/resolve')
					.then((response) => (response.ok ? response.json() : null))
					.then((resolved) => {
						if (resolved?.slug) {
							// eslint-disable-next-line svelte/no-navigation-without-resolve
							goto(`/status/${resolved.slug}`, { replaceState: true });
						} else {
							// eslint-disable-next-line svelte/no-navigation-without-resolve
							goto(`/login?returnTo=${encodeURIComponent(returnTo)}`, { replaceState: true });
						}
					})
					.catch(() => {
						// eslint-disable-next-line svelte/no-navigation-without-resolve
						goto(`/login?returnTo=${encodeURIComponent(returnTo)}`, { replaceState: true });
					});
				return;
			}
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(`/login?returnTo=${encodeURIComponent(returnTo)}`, { replaceState: true });
		}
	});

	// Track navigation depth for smart back buttons
	let lastPathname = '';
	afterNavigate((navigation) => {
		if (!navigation.to?.url) return;
		const newUrl = navigation.to.url;
		const newPathname = newUrl.pathname;

		if (navigation.type === 'enter') {
			// Full page load (direct URL or refresh): the in-app history chain is
			// gone, so smart back buttons must use their fallback path
			clearNavDepth();
		} else if (navigation.type === 'popstate') {
			// Browser back/forward button
			decrementNavDepth();
		} else if (newPathname !== lastPathname) {
			// Navigated to a different page (not just param change)
			incrementNavDepth();
		}
		// Param-only changes (same pathname) don't affect depth

		lastPathname = newPathname;

		// Make the URL the source of truth for the selected project: stamp the
		// current project onto any project-scoped URL that lacks one, so the back
		// button, reloads, and shared links all resolve to the right project.
		if (
			authState.isAuthenticated &&
			!isPublicPath(newPathname) &&
			projectsState.currentProjectId &&
			!newUrl.searchParams.get('projectId')
		) {
			const canonical = new URL(newUrl);
			canonical.searchParams.set('projectId', projectsState.currentProjectId);
			goto(resolve(canonical.pathname + canonical.search), {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	onMount(() => {
		document.getElementById('splash')?.remove();
		initTheme();
		(window as any).captureException = captureException;

		if (authState.isAuthenticated) {
			projectsState.loadProjects();
		}

		import('$billing/cross-site-notification-baner.svelte')
			.then((module) => {
				CrossSiteNotificationBanner = module.default;
			})
			.catch(() => {
				// Billing extension not available - expected for open source builds
			});
	});

	function handleLogout() {
		authState.logout();
		goto(resolve('/login'));
	}

	function handleProjectSelect(projectId: string) {
		goto(resolve(`/?projectId=${projectId}`));
	}

	function handleAddProjectClick() {
		const orgs = authState.organizations;
		if (orgs.length > 0 && orgs.every((o) => o.role === 'readonly')) {
			toast.error("You're not authorized to perform that action");
			return;
		}
		showAddProjectModal = true;
	}

	function handleProjectCreated() {
		showAddProjectModal = false;
		// Optionally navigate to connection page to show token
		goto(resolve('/connection'));
	}

	const multiOrg = $derived(authState.organizations.length > 1);

	const groupedProjects = $derived.by(() => {
		const groups: { key: string; name: string; projects: Project[] }[] = [];
		for (const org of authState.organizations) {
			const projects = projectsState.projects.filter((p) => p.organizationId === org.id);
			if (projects.length > 0) {
				groups.push({ key: `org-${org.id}`, name: org.name, projects });
			}
		}
		const knownOrgIds = new Set(authState.organizations.map((o) => o.id));
		const orphans = projectsState.projects.filter(
			(p) => !p.organizationId || !knownOrgIds.has(p.organizationId)
		);
		if (orphans.length > 0) {
			groups.push({ key: 'other', name: 'Other', projects: orphans });
		}
		return groups;
	});
</script>

<svelte:head><title>Traceway</title></svelte:head>

<svelte:window onkeydown={handleGlobalKeydown} />

{#snippet projectItem(project: Project, indent: boolean)}
	<DropdownMenu.Item
		onclick={() => handleProjectSelect(project.id)}
		class="flex cursor-pointer items-center justify-between {indent ? 'pl-4' : ''}"
	>
		<div class="flex items-center gap-2">
			<FrameworkIcon framework={project.framework} />
			<span>{project.name}</span>
		</div>
		{#if project.id === projectsState.currentProjectId}
			<Check class="h-4 w-4" />
		{/if}
	</DropdownMenu.Item>
{/snippet}

<Tooltip.Provider delayDuration={0}>
	<!-- This is not ideal, but because our layout is a top level route it can end up showing sidebar on the login page (after the login before the transition). -->
	<!-- We could consider moving this to a lower level layout for the actual app, for now it's just a path check -->
	{#if authState.isAuthenticated && !isPublicPath(page.url.pathname)}
		<Sidebar.SidebarProvider
			bind:open={sidebarOpen}
			onOpenChange={(open) => localStorage.setItem(SIDEBAR_OPEN_KEY, String(open))}
		>
			<AppSidebar />
			<Sidebar.SidebarInset>
				<header
					class="flex h-12 shrink-0 items-center gap-2 border-b bg-white px-2 dark:bg-transparent"
				>
					<Sidebar.SidebarTrigger />
					<div class="h-4 w-px bg-border"></div>
					<div class="text-lg font-semibold">
						<DropdownMenu.Root>
							<DropdownMenu.Trigger
								class="flex flex-row items-center gap-2 rounded-md px-2 py-1 transition-colors hover:bg-accent hover:text-accent-foreground"
							>
								{#if projectsState.currentProject}
									<FrameworkIcon
										framework={projectsState.currentProject.framework}
										class="size-6 shrink-0"
									/>
								{/if}
								<span>{projectsState.currentProject?.name || 'Select Project'}</span>
								<ChevronDown size={16} />
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="w-56">
								{#if multiOrg}
									{#each groupedProjects as group (group.key)}
										<DropdownMenu.Group>
											<DropdownMenu.Label class="text-xs text-muted-foreground uppercase"
												>{group.name}</DropdownMenu.Label
											>
											{#each group.projects as project (project.id)}
												{@render projectItem(project, true)}
											{/each}
										</DropdownMenu.Group>
									{/each}
								{:else}
									<DropdownMenu.Group>
										<DropdownMenu.Label>Projects</DropdownMenu.Label>
										<DropdownMenu.Separator />
										{#each projectsState.projects as project (project.id)}
											{@render projectItem(project, false)}
										{/each}
									</DropdownMenu.Group>
								{/if}
								{#if projectsState.projects.length === 0}
									<DropdownMenu.Item disabled>No projects yet</DropdownMenu.Item>
								{/if}
								<DropdownMenu.Separator />
								{#if projectsState.canManageCurrentProject}
									<DropdownMenu.Item
										onclick={() => (showEditProjectModal = true)}
										class="cursor-pointer"
									>
										<Pencil class="mr-2 h-4 w-4" />
										Edit Project
									</DropdownMenu.Item>
								{/if}
								<DropdownMenu.Item onclick={handleAddProjectClick} class="cursor-pointer">
									<Plus class="mr-2 h-4 w-4" />
									Add Project
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
					<div class="ml-auto flex items-center gap-2">
						<Button
							variant="ghost"
							size="icon"
							onclick={toggleTheme}
							title={themeState.isDark ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
						>
							{#if themeState.isDark}
								<Sun class="h-5 w-5" />
							{:else}
								<Moon class="h-5 w-5" />
							{/if}
						</Button>
						<Button variant="ghost" size="icon" onclick={handleLogout} title="Logout">
							<LogOut class="h-5 w-5" />
						</Button>
					</div>
				</header>
				<main class="min-w-0 flex-1 p-4">
					{#if timezoneMismatch}
						<WarningCallout
							class="mb-4"
							title="Your browser's timezone differs from the organization's"
						>
							All times are shown in the organization's timezone ({zoneLabel(orgZone)}). Your
							browser is on {zoneLabel(browserZone)}.
						</WarningCallout>
					{/if}
					{#if CrossSiteNotificationBanner && bannerOrganizationId !== null}
						<div class="mb-4">
							<CrossSiteNotificationBanner organizationId={bannerOrganizationId} />
						</div>
					{/if}
					{@render children()}
				</main>
			</Sidebar.SidebarInset>
		</Sidebar.SidebarProvider>

		<AddProjectModal
			open={showAddProjectModal}
			onOpenChange={(open) => (showAddProjectModal = open)}
			onProjectCreated={handleProjectCreated}
		/>

		<EditProjectModal
			open={showEditProjectModal}
			onOpenChange={(open) => (showEditProjectModal = open)}
			project={projectsState.currentProject}
		/>

		<DashboardCommand bind:open={showCommandPalette} />

		<Toaster position="top-center" />
	{:else if isPublicPath(page.url.pathname)}
		<main class="h-screen w-screen">
			{@render children()}
		</main>
	{/if}
</Tooltip.Provider>
