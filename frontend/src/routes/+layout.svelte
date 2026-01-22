<script lang="ts">
	import './layout.css';
	import { goto, afterNavigate } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { themeState, initTheme, toggleTheme } from '$lib/state/theme.svelte';
	import { initTimezone } from '$lib/state/timezone.svelte';
	import { incrementNavDepth, decrementNavDepth } from '$lib/utils/back-navigation';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import AddProjectModal from '$lib/components/add-project-modal.svelte';
	import FrameworkIcon from '$lib/components/framework-icon.svelte';
	import * as Sidebar from "$lib/components/ui/sidebar";
	import { Button } from "$lib/components/ui/button";
	import { Sun, Moon, LogOut, Plus, Check } from "@lucide/svelte";
	import { onMount } from "svelte";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { ChevronDown } from 'lucide-svelte';
	import { Toaster } from 'svelte-sonner';
	import { page } from '$app/state';

	let { children } = $props();
	let showAddProjectModal = $state(false);

	// Track navigation depth for smart back buttons
	let lastPathname = '';
	afterNavigate((navigation) => {
		if (!navigation.to?.url) return;
		const newPathname = navigation.to.url.pathname;

		if (navigation.type === 'popstate') {
			// Browser back/forward button
			decrementNavDepth();
		} else if (newPathname !== lastPathname) {
			// Navigated to a different page (not just param change)
			incrementNavDepth();
		}
		// Param-only changes (same pathname) don't affect depth

		lastPathname = newPathname;
	});

	onMount(() => {
		initTheme();
		initTimezone();

		if (authState.isAuthenticated) {
			projectsState.initFromCache();
			projectsState.loadProjects();
		}
	});

	function handleLogout() {
		authState.logout();
		goto('/login');
	}

	function handleProjectSelect(projectId: string) {
		projectsState.selectProject(projectId);
		goto('/');
	}

	function handleAddProjectClick() {
		showAddProjectModal = true;
	}

	function handleProjectCreated() {
		showAddProjectModal = false;
		// Optionally navigate to connection page to show token
		goto('/connection');
	}
</script>

<svelte:head><link rel="icon" href="/favicon.ico" /></svelte:head>

<!-- This is not ideal, but because our layout is a top level route it can end up showing sidebar on the login page (after the login before the transition). -->
<!-- We could consider moving this to a lower level layout for the actual app, for now it's just a path check -->
{#if authState.isAuthenticated && page.url.pathname !== "/login" && page.url.pathname !== "/register"}
	<Sidebar.SidebarProvider>
		<AppSidebar />
		<Sidebar.SidebarInset>
			<header class="flex h-12 shrink-0 items-center gap-2 border-b px-2">
				<Sidebar.SidebarTrigger />
				<div class="h-4 w-px bg-border"></div>
				<h1 class="text-lg font-semibold">
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class="flex flex-row items-center gap-2 hover:bg-accent hover:text-accent-foreground rounded-md px-2 py-1 transition-colors">
							{#if projectsState.currentProject}
								<FrameworkIcon framework={projectsState.currentProject.framework} />
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
										class="flex items-center justify-between cursor-pointer"
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
								<DropdownMenu.Item onclick={handleAddProjectClick} class="cursor-pointer">
									<Plus class="mr-2 h-4 w-4" />
									Add Project
								</DropdownMenu.Item>
							</DropdownMenu.Group>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</h1>
				<div class="ml-auto flex items-center gap-2">
					<Button variant="ghost" size="icon" onclick={toggleTheme} title={themeState.isDark ? "Switch to Light Mode" : "Switch to Dark Mode"}>
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
			<main class="flex-1 min-w-0 p-4">
				{@render children()}
			</main>
		</Sidebar.SidebarInset>
	</Sidebar.SidebarProvider>

	<AddProjectModal
		open={showAddProjectModal}
		onOpenChange={(open) => showAddProjectModal = open}
		onProjectCreated={handleProjectCreated}
	/>

	<Toaster position="bottom-right" />
{:else}
	<main class="h-screen w-screen">
		{@render children()}
	</main>
{/if}
