<script lang="ts">
  import * as Sidebar from "$lib/components/ui/sidebar";
  import { List, Bug, Gauge } from "@lucide/svelte";
  import { themeState } from '$lib/state/theme.svelte';
	import { LayoutDashboard } from "@lucide/svelte";
  import { page } from '$app/state';


  const sidebarItems = [
    {Icon: LayoutDashboard, href: "/", title: "Dashboard"},
    {Icon: Bug, href: "/issues", title: "Issues"},
    {Icon: List, href: "/transactions", title: "Transactions"},
  ]

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
              <a href={sidebarItem.href}>
                <Sidebar.SidebarMenuButton isActive={page.url.pathname === sidebarItem.href}>
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
