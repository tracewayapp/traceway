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

  // Reactive sidebar items with computed hrefs - explicitly read page.url to track dependency
  const sidebarItemsWithHref = $derived.by(() => {
    // Read page.url properties directly to establish reactive dependency
    const currentPath = page.url.pathname;
    const currentSearch = page.url.search;
    const preset = page.url.searchParams.get('preset');
    const from = page.url.searchParams.get('from');
    const to = page.url.searchParams.get('to');
    const servers = page.url.searchParams.get('servers');

    const isCurrentTimeRangePage = timeRangePages.some(p => currentPath.startsWith(p));

    return sidebarItems.map(item => {
      const isTargetTimeRangePage = timeRangePages.includes(item.href);

      let computedHref = item.href;

      if (isCurrentTimeRangePage && isTargetTimeRangePage) {
        const params = new URLSearchParams();

        if (preset) {
          params.set('preset', preset);
        } else if (from && to) {
          params.set('from', from);
          params.set('to', to);
        }

        // Also preserve servers param for metrics page
        if (item.href === '/metrics' && servers) {
          params.set('servers', servers);
        }

        const queryString = params.toString();
        computedHref = queryString ? `${item.href}?${queryString}` : item.href;
      }

      return { ...item, computedHref };
    });
  });

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
          {#each sidebarItemsWithHref as sidebarItem}
            <Sidebar.SidebarMenuItem>
              <a href={sidebarItem.computedHref}>
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
