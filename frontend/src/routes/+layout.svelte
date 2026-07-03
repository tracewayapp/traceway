<script lang="ts">
	import './layout.css';
	import { goto, afterNavigate } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { themeState, initTheme, toggleTheme } from '$lib/state/theme.svelte';
	import { incrementNavDepth, decrementNavDepth } from '$lib/utils/back-navigation';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import AddProjectModal from '$lib/components/add-project-modal.svelte';
	import EditProjectModal from '$lib/components/edit-project-modal.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { Button } from '$lib/components/ui/button';
	import { Sun, Moon, LogOut, Plus, Check, Pencil } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { ChevronDown } from 'lucide-svelte';
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
	let CrossSiteNotificationBanner = $state<Component<{ organizationId: number }> | null>(null);

	const bannerOrganizationId = $derived(projectsState.currentProject?.organizationId ?? null);

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

	function isProjectScopedPath(pathname: string): boolean {
		if (PUBLIC_PATHS.has(pathname)) return false;
		if (pathname.startsWith('/accept-invitation')) return false;
		return true;
	}

	// Track navigation depth for smart back buttons
	let lastPathname = '';
	afterNavigate((navigation) => {
		if (!navigation.to?.url) return;
		const newUrl = navigation.to.url;
		const newPathname = newUrl.pathname;

		if (navigation.type === 'popstate') {
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
			isProjectScopedPath(newPathname) &&
			projectsState.currentProjectId &&
			!newUrl.searchParams.get('projectId')
		) {
			const canonical = new URL(newUrl);
			canonical.searchParams.set('projectId', projectsState.currentProjectId);
			goto(canonical.pathname + canonical.search, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	onMount(() => {
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
		goto('/login');
	}

	function handleProjectSelect(projectId: string) {
		goto(`/?projectId=${projectId}`);
	}

	function handleAddProjectClick() {
		const organizationId =
			projectsState.currentProject?.organizationId ?? authState.organizations[0]?.id;
		if (organizationId) {
			const role = authState.getRoleForOrganization(organizationId);
			if (role === 'readonly') {
				toast.error("You're not authorized to perform that action", { position: 'top-center' });
				return;
			}
		}
		showAddProjectModal = true;
	}

	function handleProjectCreated() {
		showAddProjectModal = false;
		// Optionally navigate to connection page to show token
		goto('/connection');
	}
</script>

<svelte:head><title>Traceway</title></svelte:head>

<Tooltip.Provider delayDuration={0}>
<!-- This is not ideal, but because our layout is a top level route it can end up showing sidebar on the login page (after the login before the transition). -->
<!-- We could consider moving this to a lower level layout for the actual app, for now it's just a path check -->
{#if authState.isAuthenticated && page.url.pathname !== '/login' && page.url.pathname !== '/register' && page.url.pathname !== '/auth/callback' && page.url.pathname !== '/finish-setup' && page.url.pathname !== '/forgot-password' && page.url.pathname !== '/reset-password' && page.url.pathname !== '/device' && page.url.pathname !== '/oauth/authorize' && !page.url.pathname.startsWith('/accept-invitation')}
	<Sidebar.SidebarProvider>
		<AppSidebar />
		<Sidebar.SidebarInset>
			<header class="flex h-12 shrink-0 items-center gap-2 border-b px-2">
				<Sidebar.SidebarTrigger />
				<div class="h-4 w-px bg-border"></div>
				<h1 class="text-lg font-semibold">
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							class="flex flex-row items-center gap-2 rounded-md px-2 py-1 transition-colors hover:bg-accent hover:text-accent-foreground"
						>
							{#if projectsState.currentProject}
								<FrameworkIcon framework={projectsState.currentProject.framework} class="size-6 shrink-0" />
							{/if}
							<span>{projectsState.currentProject?.name || 'Select Project'}</span>
							<ChevronDown size={16} />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="start" class="w-56">
							<DropdownMenu.Group>
								<DropdownMenu.Label>Projects</DropdownMenu.Label>
								<DropdownMenu.Separator />
								{#each projectsState.projects as project}
									<DropdownMenu.Item
										onclick={() => handleProjectSelect(project.id)}
										class="flex cursor-pointer items-center justify-between"
									>
										<div class="flex items-center gap-2">
											<FrameworkIcon framework={project.framework} />
											<span>{project.name}</span>
										</div>
										{#if project.id === projectsState.currentProjectId}
											<Check class="h-4 w-4" />
										{/if}
									</DropdownMenu.Item>
								{/each}
								{#if projectsState.projects.length === 0}
									<DropdownMenu.Item disabled>No projects yet</DropdownMenu.Item>
								{/if}
								<DropdownMenu.Separator />
								{#if projectsState.canManageCurrentProject}
									<DropdownMenu.Item onclick={() => showEditProjectModal = true} class="cursor-pointer">
										<Pencil class="mr-2 h-4 w-4" />
										Edit Project
									</DropdownMenu.Item>
								{/if}
								<DropdownMenu.Item onclick={handleAddProjectClick} class="cursor-pointer">
									<Plus class="mr-2 h-4 w-4" />
									Add Project
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</h1>
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

	<Toaster position="bottom-right" />
{:else}
	{#if page.url.pathname === '/login' || page.url.pathname === '/register' || page.url.pathname === '/auth/callback' || page.url.pathname === '/finish-setup' || page.url.pathname === '/forgot-password' || page.url.pathname === '/reset-password' || page.url.pathname === '/device' || page.url.pathname === '/oauth/authorize' || page.url.pathname.startsWith('/accept-invitation')}
		<main class="h-screen w-screen">
			{@render children()}
		</main>
	{/if}
{/if}
</Tooltip.Provider>
