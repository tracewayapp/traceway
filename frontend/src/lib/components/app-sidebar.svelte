<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import { oncallState } from '$lib/state/oncall.svelte';
	import { monitorsState } from '$lib/state/monitors.svelte';
	import {
		Activity,
		Bell,
		Workflow,
		Bug,
		Link2,
		ChartNoAxesCombined,
		FileText,
		Film,
		Flame,
		Gauge,
		ListEnd,
		PhoneCall,
		Settings,
		BookOpen,
		KeyRound
	} from '@lucide/svelte';
	import { themeState } from '$lib/state/theme.svelte';
	import {
		projectsState,
		isFrontendFramework,
		isCloudflareFramework
	} from '$lib/state/projects.svelte';
	import { LayoutDashboard } from '@lucide/svelte';
	import { page } from '$app/state';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { cn } from '$lib/utils';

	interface SidebarItem {
		Icon: typeof LayoutDashboard;
		href: string;
		title: string;
		stickyParams: string[];
		adminOnly?: boolean;
		external?: boolean;
	}

	const hiddenForFrontend = new Set([
		'Dashboard',
		'Logs',
		'Endpoints',
		'Tasks',
		'Profiles',
		'Dashboards',
		'AI Traces',
		'Monitors'
	]);
	const hiddenForCloudflare = new Set(['Dashboards']);
	// Items that only make sense for frontend (browser) projects - dropped from
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
		{
			Icon: Workflow,
			href: '/ai-traces',
			title: 'AI Traces',
			stickyParams: ['preset', 'from', 'to']
		},
		{
			Icon: ChartNoAxesCombined,
			href: '/dashboards',
			title: 'Dashboards',
			stickyParams: ['preset', 'from', 'to']
		},
		{
			Icon: Activity,
			href: '/monitors',
			title: 'Monitors',
			stickyParams: ['preset', 'from', 'to']
		},
		{ Icon: Bell, href: '/notifications', title: 'Alerts', stickyParams: ['preset', 'from', 'to'] },
		{ Icon: PhoneCall, href: '/on-call', title: 'On-Call', stickyParams: [] },
		{ Icon: Link2, href: '/connection', title: 'Connection', stickyParams: [] }
	];

	const sidebarItems = $derived(
		projectsState.currentProject && isFrontendFramework(projectsState.currentProject.framework)
			? allSidebarItems.filter((item) => !hiddenForFrontend.has(item.title))
			: projectsState.currentProject &&
				  isCloudflareFramework(projectsState.currentProject.framework)
				? allSidebarItems.filter(
						(item) => !hiddenForCloudflare.has(item.title) && !frontendOnly.has(item.title)
					)
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

	// Synchronous read so the badge tracks project switches.
	$effect(() => {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		const hasMonitors = sidebarItems.some((item) => item.title === 'Monitors');
		oncallState.refreshOpenCount();
		if (hasMonitors) monitorsState.refreshDownCount();
		const interval = setInterval(() => {
			oncallState.refreshOpenCount();
			if (hasMonitors) monitorsState.refreshDownCount();
		}, 60000);
		return () => clearInterval(interval);
	});

	function navFlyoutProps(active: boolean) {
		return {
			sideOffset: 8,
			arrowClasses: 'hidden',
			class: cn(
				'flex h-9 items-center rounded-md border-border bg-popover px-3 text-sm font-medium text-popover-foreground shadow-md',
				active && 'border-transparent bg-sidebar-accent text-sidebar-accent-foreground'
			)
		};
	}
</script>

<Sidebar.Sidebar collapsible="icon" class="dark:border-white/12">
	<Sidebar.SidebarHeader
		class="flex items-start overflow-hidden p-4 transition-[padding] duration-200 ease-linear group-data-[collapsible=icon]:items-center group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-2.5"
	>
		{#if themeState.isDark}
			<img
				src="/traceway-logo-white.svg"
				alt="Traceway Logo"
				class="h-9 w-auto max-w-none shrink-0 group-data-[collapsible=icon]:hidden"
			/>
		{:else}
			<img
				src="/traceway-logo.png"
				alt="Traceway Logo"
				class="h-9 w-auto max-w-none shrink-0 group-data-[collapsible=icon]:hidden"
			/>
		{/if}
		<img
			src="/traceway-mark.png"
			alt="Traceway Logo"
			class="mt-1 hidden size-9 shrink-0 object-contain group-data-[collapsible=icon]:block dark:invert"
		/>
	</Sidebar.SidebarHeader>
	<Sidebar.SidebarContent>
		<Sidebar.SidebarGroup
			class="p-4 pt-0 pb-0 transition-[padding] duration-200 ease-linear group-data-[collapsible=icon]:px-2.5 group-data-[collapsible=icon]:pt-2"
		>
			<Sidebar.SidebarGroupContent>
				<Sidebar.SidebarMenu>
					{#each sidebarItems as sidebarItem, __index (__index)}
						{@const active =
							page.url.pathname === sidebarItem.href ||
							page.url.pathname.startsWith(sidebarItem.href + '/')}
						<Sidebar.SidebarMenuItem>
							<Sidebar.SidebarMenuButton
								isActive={active}
								tooltipContent={sidebarItem.title}
								tooltipContentProps={navFlyoutProps(active)}
								onclick={(e) => {
									sidebar.setOpenMobile(false);
									createRowClickHandler(sidebarItem.href, ...sidebarItem.stickyParams)(e);
								}}
							>
								<sidebarItem.Icon />
								<span>{sidebarItem.title}</span>
							</Sidebar.SidebarMenuButton>
							{#if sidebarItem.title === 'Monitors' && monitorsState.downChecksCount > 0}
								<Sidebar.MenuBadge
									class="rounded-full bg-destructive text-[10px] text-white peer-hover/menu-button:text-white peer-data-[active=true]/menu-button:text-white"
								>
									{monitorsState.downChecksCount}
								</Sidebar.MenuBadge>
							{/if}
							{#if sidebarItem.title === 'On-Call' && oncallState.openPagesCount > 0}
								<Sidebar.MenuBadge
									class="rounded-full bg-destructive text-[10px] text-white peer-hover/menu-button:text-white peer-data-[active=true]/menu-button:text-white"
								>
									{oncallState.openPagesCount}
								</Sidebar.MenuBadge>
							{/if}
						</Sidebar.SidebarMenuItem>
					{/each}
				</Sidebar.SidebarMenu>
			</Sidebar.SidebarGroupContent>
		</Sidebar.SidebarGroup>

		{#if sidebarItemsBottom.length}
			<div class="flex-1"></div>

			<Sidebar.SidebarGroup
				class="p-4 pt-0 transition-[padding] duration-200 ease-linear group-data-[collapsible=icon]:px-2.5 group-data-[collapsible=icon]:pb-2.5"
			>
				<Sidebar.SidebarGroupContent>
					<Sidebar.SidebarMenu>
						{#each sidebarItemsBottom as sidebarItem, __index (__index)}
							<Sidebar.SidebarMenuItem>
								{#if sidebarItem.external}
									<Sidebar.SidebarMenuButton
										tooltipContent={sidebarItem.title}
										tooltipContentProps={navFlyoutProps(false)}
										onclick={() => {
											sidebar.setOpenMobile(false);
											window.open(sidebarItem.href, '_blank', 'noopener,noreferrer');
										}}
									>
										<sidebarItem.Icon />
										<span>{sidebarItem.title}</span>
									</Sidebar.SidebarMenuButton>
								{:else}
									{@const active =
										page.url.pathname === sidebarItem.href ||
										page.url.pathname.startsWith(sidebarItem.href + '/')}
									<Sidebar.SidebarMenuButton
										isActive={active}
										tooltipContent={sidebarItem.title}
										tooltipContentProps={navFlyoutProps(active)}
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
		<div class="hidden group-data-[collapsible=icon]:block">
			<span class="text-xs text-muted-foreground">&nbsp;</span>
		</div>

		<div class="group-data-[collapsible=icon]:hidden">
			<span class="text-xs text-muted-foreground">Traceway - v{__APP_VERSION__}</span>
		</div>
	</Sidebar.SidebarFooter>
</Sidebar.Sidebar>
