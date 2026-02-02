<script lang="ts">
  import * as Sidebar from "$lib/components/ui/sidebar";
  import { useSidebar } from '$lib/components/ui/sidebar';
  import { Bug, Link2, ChartNoAxesCombined, ChartNoAxesGantt, Gauge, ListEnd, Settings, BookOpen } from "@lucide/svelte";
  import { themeState } from '$lib/state/theme.svelte';
  import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import { LayoutDashboard } from "@lucide/svelte";
  import { page } from '$app/state';
	import { createRowClickHandler } from "$lib/utils/navigation";

  interface SidebarItem {
    Icon: typeof LayoutDashboard;
    href: string;
    title: string;
    stickyParams: string[];
    adminOnly?: boolean;
    external?: boolean;
  }

  const hiddenForFrontend = new Set(['Endpoints', 'Tasks', 'Metrics']);

  const allSidebarItems: SidebarItem[] = [
    {Icon: LayoutDashboard, href: "/", title: "Dashboard", stickyParams: []},
    {Icon: Bug, href: "/issues", title: "Issues", stickyParams: []},
    {Icon: Gauge, href: "/endpoints", title: "Endpoints", stickyParams: ['presets', 'from', 'to']},
    {Icon: ListEnd, href: "/tasks", title: "Tasks", stickyParams: ['presets', 'from', 'to']},
    {Icon: ChartNoAxesCombined, href: "/metrics", title: "Metrics", stickyParams: ['presets', 'from', 'to']},
    {Icon: Link2, href: "/connection", title: "Connection", stickyParams: []},
  ]

  const sidebarItems = $derived(
    projectsState.currentProject && isFrontendFramework(projectsState.currentProject.framework)
      ? allSidebarItems.filter(item => !hiddenForFrontend.has(item.title))
      : allSidebarItems
  );

  const allSidebarItemsBottom: SidebarItem[] = [
    {Icon: BookOpen, href: "https://docs.tracewayapp.com", title: "Docs", stickyParams: [], external: true},
    {Icon: Settings, href: "/settings", title: "Settings", stickyParams: [], adminOnly: true},
  ]

  const sidebarItemsBottom = $derived(
    allSidebarItemsBottom.filter(item => !item.adminOnly || projectsState.canManageCurrentProject)
  );

  const sidebar = useSidebar();
</script>

<Sidebar.Sidebar>
  <Sidebar.SidebarHeader class="p-4 flex items-start">
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
              <Sidebar.SidebarMenuButton isActive={page.url.pathname === sidebarItem.href || page.url.pathname.startsWith(sidebarItem.href + '/')} onclick={(e) => {
                sidebar.setOpenMobile(false)
                createRowClickHandler(sidebarItem.href, ...sidebarItem.stickyParams)(e)
              }}>
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
                  <Sidebar.SidebarMenuButton onclick={() => {
                    sidebar.setOpenMobile(false)
                    window.open(sidebarItem.href, '_blank', 'noopener,noreferrer')
                  }}>
                    <sidebarItem.Icon />
                    <span>{sidebarItem.title}</span>
                  </Sidebar.SidebarMenuButton>
                {:else}
                  <Sidebar.SidebarMenuButton isActive={page.url.pathname === sidebarItem.href || page.url.pathname.startsWith(sidebarItem.href + '/')} onclick={(e) => {
                    sidebar.setOpenMobile(false)
                    createRowClickHandler(sidebarItem.href, ...sidebarItem.stickyParams)(e)
                  }}>
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
  <Sidebar.SidebarFooter class="py-1 border-t border-border flex flex-row justify-center italic">
    <span class="text-xs text-muted-foreground">Traceway - v{__APP_VERSION__}</span>
  </Sidebar.SidebarFooter>
</Sidebar.Sidebar>
