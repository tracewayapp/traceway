<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import {
		Bell,
		Workflow,
		Bug,
		Link2,
		ChartNoAxesCombined,
		ChartNoAxesGantt,
		FileText,
		Film,
		Flame,
		Gauge,
		ListEnd,
		Settings,
		BookOpen,
		KeyRound
	} from '@lucide/svelte';
	import { themeState } from '$lib/state/theme.svelte';
	import { projectsState, isFrontendFramework, isCloudflareFramework } from '$lib/state/projects.svelte';
	import { LayoutDashboard } from '@lucide/svelte';
	import { page } from '$app/state';
	import { createRowClickHandler } from '$lib/utils/navigation';

	interface SidebarItem {
		Icon: typeof LayoutDashboard;
		href: string;
		title: string;
		stickyParams: string[];
		adminOnly?: boolean;
		external?: boolean;
	}

	const hiddenForFrontend = new Set(['Dashboard', 'Logs', 'Endpoints', 'Tasks', 'Profiles', 'Metrics', 'AI Traces']);
	const hiddenForCloudflare = new Set(['Metrics']);
	// Items that only make sense for frontend (browser) projects — dropped from
	// the sidebar for any other framework, so a Go backend project doesn't
	// surface a Sessions tab that can never be populated.
	const frontendOnly = new Set(['Sessions']);

	const allSidebarItems: SidebarItem[] = [
		{ Icon: LayoutDashboard, href: '/', title: 'Dashboard', stickyParams: [] },
		{ Icon: Bug, href: '/issues', title: 'Issues', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: FileText, href: '/logs', title: 'Logs', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: Gauge, href: '/endpoints', title: 'Endpoints', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: ListEnd, href: '/tasks', title: 'Tasks', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: Flame, href: '/profiles', title: 'Profiles', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: Film, href: '/sessions', title: 'Sessions', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: Workflow, href: '/ai-traces', title: 'AI Traces', stickyParams: ['preset', 'from', 'to'] },
		{
			Icon: ChartNoAxesCombined,
			href: '/metrics',
			title: 'Metrics',
			stickyParams: ['preset', 'from', 'to']
		},
		{ Icon: Bell, href: '/notifications', title: 'Alerts', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: Link2, href: '/connection', title: 'Connection', stickyParams: [] }
	];

	const sidebarItems = $derived(
		projectsState.currentProject && isFrontendFramework(projectsState.currentProject.framework)
			? allSidebarItems.filter((item) => !hiddenForFrontend.has(item.title))
			: projectsState.currentProject && isCloudflareFramework(projectsState.currentProject.framework)
				? allSidebarItems.filter((item) => !hiddenForCloudflare.has(item.title) && !frontendOnly.has(item.title))
				: allSidebarItems.filter((item) => !frontendOnly.has(item.title))
	);

	const allSidebarItemsBottom: SidebarItem[] = [
		{ Icon: KeyRound, href: '/account', title: 'Account', stickyParams: [] },
		{
			Icon: BookOpen,
			href: 'https://docs.tracewayapp.com',
			title: 'Docs',
			stickyParams: [],
			external: true
		},
		{ Icon: Settings, href: '/settings', title: 'Settings', stickyParams: [], adminOnly: true }
	];

	const sidebarItemsBottom = $derived(
		allSidebarItemsBottom.filter((item) => !item.adminOnly || projectsState.canManageCurrentProject)
	);

	const sidebar = useSidebar();
</script>

<Sidebar.Sidebar>
	<Sidebar.SidebarHeader class="flex items-start p-4">
		{#if themeState.isDark}
			<img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-9 w-auto" />
		{:else}
			<img src="/traceway-logo.png" alt="Traceway Logo" class="h-9 w-auto" />
		{/if}
	</Sidebar.SidebarHeader>
	<Sidebar.SidebarContent>
		<Sidebar.SidebarGroup class="p-4 pt-0 pb-0">
			<Sidebar.SidebarGroupContent>
				<Sidebar.SidebarMenu>
					{#each sidebarItems as sidebarItem}
						<Sidebar.SidebarMenuItem>
							<Sidebar.SidebarMenuButton
								isActive={page.url.pathname === sidebarItem.href ||
									page.url.pathname.startsWith(sidebarItem.href + '/')}
								onclick={(e) => {
									sidebar.setOpenMobile(false);
									createRowClickHandler(sidebarItem.href, ...sidebarItem.stickyParams)(e);
								}}
							>
								<sidebarItem.Icon />
								<span>{sidebarItem.title}</span>
							</Sidebar.SidebarMenuButton>
						</Sidebar.SidebarMenuItem>
					{/each}
				</Sidebar.SidebarMenu>
			</Sidebar.SidebarGroupContent>
		</Sidebar.SidebarGroup>

		{#if sidebarItemsBottom.length}
			<div class="flex-1"></div>

			<Sidebar.SidebarGroup class="p-4 pt-0">
				<Sidebar.SidebarGroupContent>
					<Sidebar.SidebarMenu>
						{#each sidebarItemsBottom as sidebarItem}
							<Sidebar.SidebarMenuItem>
								{#if sidebarItem.external}
									<Sidebar.SidebarMenuButton
										onclick={() => {
											sidebar.setOpenMobile(false);
											window.open(sidebarItem.href, '_blank', 'noopener,noreferrer');
										}}
									>
										<sidebarItem.Icon />
										<span>{sidebarItem.title}</span>
									</Sidebar.SidebarMenuButton>
								{:else}
									<Sidebar.SidebarMenuButton
										isActive={page.url.pathname === sidebarItem.href ||
											page.url.pathname.startsWith(sidebarItem.href + '/')}
										onclick={(e) => {
											sidebar.setOpenMobile(false);
											createRowClickHandler(sidebarItem.href, ...sidebarItem.stickyParams)(e);
										}}
									>
										<sidebarItem.Icon />
										<span>{sidebarItem.title}</span>
									</Sidebar.SidebarMenuButton>
								{/if}
							</Sidebar.SidebarMenuItem>
						{/each}
					</Sidebar.SidebarMenu>
				</Sidebar.SidebarGroupContent>
			</Sidebar.SidebarGroup>
		{/if}
	</Sidebar.SidebarContent>
	<Sidebar.SidebarFooter class="flex flex-row justify-center border-t border-border py-1 italic">
		<span class="text-xs text-muted-foreground">Traceway - v{__APP_VERSION__}</span>
	</Sidebar.SidebarFooter>
</Sidebar.Sidebar>
