<script lang="ts">
  import * as Sidebar from "$lib/components/ui/sidebar";
  import { List, Bug, Link2, BarChart3 } from "@lucide/svelte";
  import { themeState } from '$lib/state/theme.svelte';
	import { LayoutDashboard } from "@lucide/svelte";
  import { page } from '$app/state';

  // Pages that share time range params
  const timeRangePages = ['/metrics', '/transactions'];

  const sidebarItems = [
    {Icon: LayoutDashboard, href: "/", title: "Dashboard"},
    {Icon: Bug, href: "/issues", title: "Issues"},
    {Icon: List, href: "/transactions", title: "Transactions"},
    {Icon: BarChart3, href: "/metrics", title: "Metrics"},
    {Icon: Link2, href: "/connection", title: "Connection"},
  ]

  // Build href with time range params preserved between metrics and transactions
  function getHref(baseHref: string): string {
    const currentPath = page.url.pathname;

    // Only preserve params when navigating between time range pages
    const isCurrentTimeRangePage = timeRangePages.some(p => currentPath.startsWith(p));
    const isTargetTimeRangePage = timeRangePages.includes(baseHref);

    if (isCurrentTimeRangePage && isTargetTimeRangePage) {
      // Preserve preset, from, to params
      const params = new URLSearchParams();
      const preset = page.url.searchParams.get('preset');
      const from = page.url.searchParams.get('from');
      const to = page.url.searchParams.get('to');

      if (preset) {
        params.set('preset', preset);
      } else if (from && to) {
        params.set('from', from);
        params.set('to', to);
      }

      // Also preserve servers param for metrics page
      if (baseHref === '/metrics') {
        const servers = page.url.searchParams.get('servers');
        if (servers) params.set('servers', servers);
      }

      const queryString = params.toString();
      return queryString ? `${baseHref}?${queryString}` : baseHref;
    }

    return baseHref;
  }

</script>

<Sidebar.Sidebar>
  <Sidebar.SidebarHeader class="p-4 flex items-start">
    {#if themeState.isDark}
      <img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-10 w-auto" />
    {:else}
      <img src="/traceway-logo.png" alt="Traceway Logo" class="h-10 w-auto" />
    {/if}
  </Sidebar.SidebarHeader>
  <Sidebar.SidebarContent>
    <Sidebar.SidebarGroup>
      <Sidebar.SidebarGroupContent>
        <Sidebar.SidebarMenu>
          {#each sidebarItems as sidebarItem}
            <Sidebar.SidebarMenuItem>
              <a href={getHref(sidebarItem.href)}>
                <Sidebar.SidebarMenuButton isActive={page.url.pathname === sidebarItem.href || page.url.pathname.startsWith(sidebarItem.href + '/')}>
                  <sidebarItem.Icon />
                  <span>{sidebarItem.title}</span>
                </Sidebar.SidebarMenuButton>
              </a>
            </Sidebar.SidebarMenuItem>
          {/each}
        </Sidebar.SidebarMenu>
      </Sidebar.SidebarGroupContent>
    </Sidebar.SidebarGroup>
  </Sidebar.SidebarContent>
</Sidebar.Sidebar>
