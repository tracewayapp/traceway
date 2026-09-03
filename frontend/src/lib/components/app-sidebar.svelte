<script lang="ts">
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { useSidebar } from '$lib/components/ui/sidebar';
	import { oncallState } from '$lib/state/oncall.svelte';
	import { monitorsState } from '$lib/state/monitors.svelte';
	import { organizationContext } from '$lib/state/organization-context.svelte';
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
		FolderKanban,
		Gauge,
		ListEnd,
		PhoneCall,
		Server,
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
	import { addStickyParamsToHref, createRowClickHandler } from '$lib/utils/navigation';
	import { resolveHref } from '$lib/utils/links';
	import { cn } from '$lib/utils';

	interface SidebarItem {
		Icon: typeof LayoutDashboard;
		href: string;
		title: string;
		stickyParams: string[];
		adminOnly?: boolean;
		external?: boolean;
		exact?: boolean;
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

	const allOrganizationItems: SidebarItem[] = [
		{
			Icon: Server,
			href: '/organization',
			title: 'Servers',
			stickyParams: ['organizationId'],
			exact: true
		},
		{ Icon: Bug, href: '/organization/issues', title: 'Issues', stickyParams: ['organizationId'] },
		{
			Icon: Activity,
			href: '/organization/monitors',
			title: 'Monitors',
			stickyParams: ['organizationId']
		},
		{
			Icon: PhoneCall,
			href: '/organization/on-call',
			title: 'On-Call',
			stickyParams: ['organizationId']
		},
		{
			Icon: FolderKanban,
			href: '/organization/projects',
			title: 'Projects',
			stickyParams: ['organizationId']
		}
	];
	const backendOnlyOrganizationItems = new Set(['Servers', 'Monitors']);

	const organizationItems = $derived(
		organizationContext.hasBackendProjects
			? allOrganizationItems
			: allOrganizationItems.filter((item) => !backendOnlyOrganizationItems.has(item.title))
	);

	const items = $derived(organizationContext.active ? organizationItems : sidebarItems);

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

	const organizationProjectParam = $derived.by(() => {
		const project = organizationContext.projects[0];
		return project ? `?projectId=${project.id}` : '';
	});

	const itemsBottom = $derived.by((): SidebarItem[] => {
		if (!organizationContext.active) return sidebarItemsBottom;
		return allSidebarItemsBottom
			.filter((item) => !item.adminOnly || organizationContext.canManage)
			.map((item) =>
				item.external ? item : { ...item, href: item.href + organizationProjectParam }
			);
	});

	const sidebar = useSidebar();

	const homeItem = $derived(items[0]);
	const homeHref = $derived.by(() => {
		void page.url;
		return homeItem ? addStickyParamsToHref(homeItem.href, ...homeItem.stickyParams) : '/';
	});

	function goHome(event: MouseEvent) {
		event.preventDefault();
		if (!homeItem) return;
		sidebar.setOpenMobile(false);
		createRowClickHandler(homeItem.href, ...homeItem.stickyParams)(event);
	}

	function isActive(item: SidebarItem): boolean {
		if (page.url.pathname === item.href) return true;
		if (item.exact) return false;
		return page.url.pathname.startsWith(item.href + '/');
	}

	function badgeCount(item: SidebarItem): number {
		if (item.title === 'Monitors') {
			return organizationContext.active
				? organizationContext.downMonitorsCount
				: monitorsState.downChecksCount;
		}
		if (item.title === 'On-Call') {
			return organizationContext.active
				? organizationContext.openPagesCount
				: oncallState.openPagesCount;
		}
		return 0;
	}

	// Synchronous read so the badge tracks project switches.
	$effect(() => {
		if (organizationContext.active) return;
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

	$effect(() => {
		if (!organizationContext.active) return;
		const organizationId = organizationContext.organizationId;
		if (organizationId === null) return;
		organizationContext.refreshCounts(organizationId);
		const interval = setInterval(() => organizationContext.refreshCounts(organizationId), 60000);
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
		<a
			{...{ href: resolveHref(homeHref) }}
			onclick={goHome}
			aria-label={homeItem ? `Go to ${homeItem.title}` : 'Home'}
			data-testid="sidebar-home"
			class="flex shrink-0 rounded-md focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none"
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
		</a>
	</Sidebar.SidebarHeader>
	<Sidebar.SidebarContent>
		<Sidebar.SidebarGroup
			class="p-4 pt-0 pb-0 transition-[padding] duration-200 ease-linear group-data-[collapsible=icon]:px-2.5 group-data-[collapsible=icon]:pt-2"
		>
			{#if organizationContext.active}
				<Sidebar.SidebarGroupLabel
					class="mb-1 text-[11px] font-semibold tracking-[0.14em] text-muted-foreground uppercase"
				>
					Organization
				</Sidebar.SidebarGroupLabel>
			{/if}
			<Sidebar.SidebarGroupContent>
				<Sidebar.SidebarMenu>
					{#each items as sidebarItem (sidebarItem.href)}
						{@const active = isActive(sidebarItem)}
						{@const badge = badgeCount(sidebarItem)}
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
							{#if badge > 0}
								<Sidebar.MenuBadge
									class="rounded-full bg-destructive text-[10px] text-white peer-hover/menu-button:text-white peer-data-[active=true]/menu-button:text-white"
								>
									{badge}
								</Sidebar.MenuBadge>
							{/if}
						</Sidebar.SidebarMenuItem>
					{/each}
				</Sidebar.SidebarMenu>
			</Sidebar.SidebarGroupContent>
		</Sidebar.SidebarGroup>

		{#if itemsBottom.length}
			<div class="flex-1"></div>

			<Sidebar.SidebarGroup
				class="p-4 pt-0 transition-[padding] duration-200 ease-linear group-data-[collapsible=icon]:px-2.5 group-data-[collapsible=icon]:pb-2.5"
			>
				<Sidebar.SidebarGroupContent>
					<Sidebar.SidebarMenu>
						{#each itemsBottom as sidebarItem (sidebarItem.title)}
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
									{@const active = isActive(sidebarItem)}
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
