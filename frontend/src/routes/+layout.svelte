<script lang="ts">
	import { resolve } from '$app/paths';
	import './layout.css';
	import { goto, afterNavigate } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { themeState, initTheme, toggleTheme } from '$lib/state/theme.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { DateTime } from 'luxon';
	import { incrementNavDepth, decrementNavDepth, clearNavDepth } from '$lib/utils/back-navigation';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import AddProjectModal from '$lib/components/add-project-modal.svelte';
	import EditProjectModal from '$lib/components/edit-project-modal.svelte';
	import DashboardCommand from '$lib/components/dashboard/dashboard-command.svelte';
	import OrganizationProjectSwitcher from '$lib/components/organization-project-switcher.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Button } from '$lib/components/ui/button';
	import { Sun, Moon, LogOut } from '@lucide/svelte';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import { onMount } from 'svelte';
	import { Toaster, toast } from 'svelte-sonner';
	import { page } from '$app/state';
	import { defaultAuthenticatedPath, gotoHref } from '$lib/utils/navigation';
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
			if (page.url.pathname === '/dashboards' || isOrganizationPath(page.url.pathname)) return;
			e.preventDefault();
			showCommandPalette = !showCommandPalette;
		}
	}

	const SIDEBAR_OPEN_KEY = 'traceway_sidebar_open';
	let sidebarOpen = $state(localStorage.getItem(SIDEBAR_OPEN_KEY) !== 'false');
	let CrossSiteNotificationBanner = $state<Component<{ organizationId: number }> | null>(null);

	function isOrganizationPath(pathname: string): boolean {
		return pathname === '/organization' || pathname.startsWith('/organization/');
	}

	const organizationPage = $derived(isOrganizationPath(page.url.pathname));
	const selectedOrganizationId = $derived.by(() => {
		if (!organizationPage) return null;
		const value = Number(page.url.searchParams.get('organizationId'));
		if (
			Number.isInteger(value) &&
			authState.organizations.some((organization) => organization.id === value)
		) {
			return value;
		}
		return authState.organizations[0]?.id ?? null;
	});
	const contextOrganizationId = $derived(
		organizationPage
			? selectedOrganizationId
			: (projectsState.currentProject?.organizationId ?? null)
	);
	const bannerOrganizationId = $derived(contextOrganizationId);

	// Warn when the browser's timezone renders times differently from the
	// organization's timezone (all timestamps are shown in the org timezone).
	// Compare offsets, not zone names — Europe/Berlin vs Europe/Belgrade is
	// not a mismatch worth warning about.
	const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	const orgZone = $derived(
		(contextOrganizationId ? authState.getTimezoneForOrganization(contextOrganizationId) : null) ||
			getTimezone()
	);
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
							goto(resolve(`/status/${resolved.slug}` as '/'), { replaceState: true });
						} else {
							gotoHref(`/login?returnTo=${encodeURIComponent(returnTo)}`, {
								replaceState: true
							});
						}
					})
					.catch(() => {
						gotoHref(`/login?returnTo=${encodeURIComponent(returnTo)}`, {
							replaceState: true
						});
					});
				return;
			}
			gotoHref(`/login?returnTo=${encodeURIComponent(returnTo)}`, {
				replaceState: true
			});
		}
	});

	// Zero-project accounts have no dashboard to show: they are locked to the
	// standalone /setup flow until their first project exists.
	const needsSetup = $derived(authState.isAuthenticated && projectsState.projects.length === 0);

	const inSetupFlow = $derived(needsSetup || page.url.pathname === '/setup');

	$effect(() => {
		if (needsSetup && !isPublicPath(page.url.pathname) && page.url.pathname !== '/setup') {
			goto(resolve('/setup'), { replaceState: true });
		}
	});

	$effect(() => {
		if (
			!authState.isAuthenticated ||
			page.url.pathname !== '/' ||
			page.url.searchParams.has('projectId') ||
			projectsState.projects.length === 0
		) {
			return;
		}
		const destination = defaultAuthenticatedPath(authState.organizations, projectsState.projects);
		if (destination !== '/') {
			gotoHref(destination, { replaceState: true });
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
			!isOrganizationPath(newPathname) &&
			projectsState.currentProjectId &&
			!newUrl.searchParams.get('projectId')
		) {
			const canonical = new URL(newUrl);
			canonical.searchParams.set('projectId', projectsState.currentProjectId);
			gotoHref(canonical.pathname + canonical.search, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	onMount(() => {
		document.getElementById('splash')?.remove();
		initTheme();
		window.captureException = captureException;

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
		projectsState.clear();
		goto(resolve('/login'));
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
		goto(resolve('/connection'));
	}
</script>

<svelte:head><title>Traceway</title></svelte:head>

<svelte:window onkeydown={handleGlobalKeydown} />

{#snippet accountActions()}
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
{/snippet}

{#snippet contextNotices()}
	{#if timezoneMismatch}
		<WarningCallout class="mb-4" title="Your browser's timezone differs from the organization's">
			All times are shown in the organization's timezone ({zoneLabel(orgZone)}). Your browser is on {zoneLabel(
				browserZone
			)}.
		</WarningCallout>
	{/if}
	{#if CrossSiteNotificationBanner && bannerOrganizationId !== null}
		<div class="mb-4">
			<CrossSiteNotificationBanner organizationId={bannerOrganizationId} />
		</div>
	{/if}
{/snippet}

{#snippet authenticatedOverlays()}
	<AddProjectModal
		open={showAddProjectModal}
		onOpenChange={(open) => (showAddProjectModal = open)}
		onProjectCreated={handleProjectCreated}
		initialOrganizationId={organizationPage ? selectedOrganizationId : null}
	/>

	<EditProjectModal
		open={showEditProjectModal}
		onOpenChange={(open) => (showEditProjectModal = open)}
		project={projectsState.currentProject}
	/>

	<DashboardCommand bind:open={showCommandPalette} />
	<Toaster position="top-center" />
{/snippet}

<Tooltip.Provider delayDuration={0}>
	<!-- This is not ideal, but because our layout is a top level route it can end up showing sidebar on the login page (after the login before the transition). -->
	<!-- We could consider moving this to a lower level layout for the actual app, for now it's just a path check -->
	{#if authState.isAuthenticated && !isPublicPath(page.url.pathname) && inSetupFlow}
		<div class="flex min-h-screen flex-col">
			<header class="flex h-14 shrink-0 items-center justify-between px-4">
				{#if themeState.isDark}
					<img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-7 w-auto" />
				{:else}
					<img src="/traceway-logo.png" alt="Traceway Logo" class="h-7 w-auto" />
				{/if}
				{@render accountActions()}
			</header>
			<main class="flex flex-1 justify-center px-4 py-10">
				{@render children()}
			</main>
		</div>
		<Toaster position="top-center" />
	{:else if authState.isAuthenticated && !isPublicPath(page.url.pathname) && organizationPage}
		<div class="min-h-screen bg-background">
			<header
				class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b bg-background/95 px-3 backdrop-blur supports-[backdrop-filter]:bg-background/80 sm:px-4"
			>
				{#if themeState.isDark}
					<img
						src="/traceway-logo-white.svg"
						alt="Traceway Logo"
						class="hidden h-7 w-auto shrink-0 sm:block"
					/>
				{:else}
					<img
						src="/traceway-logo.png"
						alt="Traceway Logo"
						class="hidden h-7 w-auto shrink-0 sm:block"
					/>
				{/if}
				<img
					src="/traceway-mark.png"
					alt="Traceway Logo"
					class="size-7 shrink-0 object-contain sm:hidden dark:invert"
				/>
				<div class="mx-1 h-5 w-px shrink-0 bg-border"></div>
				<OrganizationProjectSwitcher
					onAddProject={handleAddProjectClick}
					onEditProject={() => (showEditProjectModal = true)}
				/>
				{@render accountActions()}
			</header>
			<main class="mx-auto w-full max-w-[1600px] p-4 sm:p-6">
				{@render contextNotices()}
				{@render children()}
			</main>
		</div>
		{@render authenticatedOverlays()}
	{:else if authState.isAuthenticated && !isPublicPath(page.url.pathname)}
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
					<OrganizationProjectSwitcher
						onAddProject={handleAddProjectClick}
						onEditProject={() => (showEditProjectModal = true)}
					/>
					{@render accountActions()}
				</header>
				<main class="min-w-0 flex-1 p-4">
					{@render contextNotices()}
					{@render children()}
				</main>
			</Sidebar.SidebarInset>
		</Sidebar.SidebarProvider>
		{@render authenticatedOverlays()}
	{:else if isPublicPath(page.url.pathname)}
		<main class="h-screen w-screen">
			{@render children()}
		</main>
	{/if}
</Tooltip.Provider>
