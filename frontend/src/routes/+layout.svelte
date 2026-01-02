<script lang="ts">
	import './layout.css';
	import { goto } from '$app/navigation';
	import { authState } from '$lib/state/auth.svelte';
	import { themeState, initTheme, toggleTheme } from '$lib/state/theme.svelte';
	import AppSidebar from '$lib/components/app-sidebar.svelte';
	import * as Sidebar from "$lib/components/ui/sidebar";
	import { Button } from "$lib/components/ui/button";
	import { Sun, Moon, LogOut } from "@lucide/svelte";
	import { onMount } from "svelte";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import { ChevronDown } from 'lucide-svelte';

	let { children } = $props();

	onMount(() => {
		return initTheme();
	});

	function handleLogout() {
		authState.logout();
		goto('/login');
	}
</script>

<svelte:head><link rel="icon" href="/favicon.ico" /></svelte:head>

{#if authState.isAuthenticated}
	<Sidebar.SidebarProvider>
		<AppSidebar />
		<Sidebar.SidebarInset>
			<header class="flex h-16 shrink-0 items-center gap-2 border-b px-4">
				<Sidebar.SidebarTrigger />
				<div class="h-4 w-px bg-border"></div>
				<h1 class="text-lg font-semibold">
					<DropdownMenu.Root>
						<DropdownMenu.Trigger class="flex flex-row items-center">
							<div>Project 1</div> <ChevronDown size=16 />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content>
							<DropdownMenu.Group>
								<DropdownMenu.Label>Projects</DropdownMenu.Label>
								<DropdownMenu.Separator />
								<DropdownMenu.Item>CT gin-backend</DropdownMenu.Item>
								<DropdownMenu.Item>CT eldapp</DropdownMenu.Item>
								<DropdownMenu.Item>CT frontend</DropdownMenu.Item>
								<DropdownMenu.Item>Traceway BE</DropdownMenu.Item>
								<DropdownMenu.Separator />
								<DropdownMenu.Item>+ Add Project</DropdownMenu.Item>
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
			<main class="flex-1 p-4">
				{@render children()}
			</main>
		</Sidebar.SidebarInset>
	</Sidebar.SidebarProvider>
{:else}
	<main class="h-screen w-screen">
		{@render children()}
	</main>
{/if}
