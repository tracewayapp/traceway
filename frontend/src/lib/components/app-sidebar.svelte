<script lang="ts">
  import * as Sidebar from "$lib/components/ui/sidebar";
  import { useSidebar } from '$lib/components/ui/sidebar';
  import { Bug, Link2, ChartLine } from "@lucide/svelte";
  import { themeState } from '$lib/state/theme.svelte';
	import { LayoutDashboard } from "@lucide/svelte";
  import { page } from '$app/state';
	import { createRowClickHandler } from "$lib/utils/navigation";
	import { Gauge, ListTodo } from "lucide-svelte";

  const sidebarItems = [
    {Icon: LayoutDashboard, href: "/", title: "Dashboard", stickyParams: [] as string[]},
    {Icon: Bug, href: "/issues", title: "Issues", stickyParams: [] as string[]},
    {Icon: Gauge, href: "/endpoints", title: "Endpoints", stickyParams: ['presets', 'from', 'to']},
    {Icon: ListTodo, href: "/tasks", title: "Tasks", stickyParams: ['presets', 'from', 'to']},
    {Icon: ChartLine, href: "/metrics", title: "Metrics", stickyParams: ['presets', 'from', 'to']},
    {Icon: Link2, href: "/connection", title: "Connection", stickyParams: [] as string[]},
  ]

  const sidebar = useSidebar();
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
  </Sidebar.SidebarContent>
</Sidebar.Sidebar>
