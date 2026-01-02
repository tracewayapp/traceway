<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { RefreshCw } from 'lucide-svelte';
	import MetricCard from '$lib/components/dashboard/metric-card.svelte';
	import { generateMockDashboardData } from '$lib/utils/mock-dashboard-data';
	import type { DashboardData } from '$lib/types/dashboard';
	import { Card, CardContent } from '$lib/components/ui/card';

	let dashboardData = $state<DashboardData | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function loadDashboard() {
		loading = true;
		error = '';

		try {
			// Simulate API delay
			await new Promise((resolve) => setTimeout(resolve, 500));

			// Mock data - TODO: Replace with real API call
			// const response = await api.get('/metrics/dashboard');
			// dashboardData = response;
			dashboardData = generateMockDashboardData();
		} catch (e: any) {
			error = e.message || 'Failed to load dashboard data';
			console.error(e);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadDashboard();
	});

	const lastUpdatedFormatted = $derived(
		dashboardData?.lastUpdated
			? dashboardData.lastUpdated.toLocaleString('en-US', {
					hour: '2-digit',
					minute: '2-digit',
					second: '2-digit',
					hour12: false
				})
			: ''
	);
</script>

<div class="space-y-4">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-3xl font-bold tracking-tight">System Metrics</h2>
			{#if dashboardData}
				<p class="mt-1 text-sm text-muted-foreground">Last updated: {lastUpdatedFormatted}</p>
			{/if}
		</div>
		<Button variant="outline" size="sm" onclick={loadDashboard} disabled={loading}>
			<RefreshCw class="mr-2 h-4 w-4 {loading ? 'animate-spin' : ''}" />
			Refresh
		</Button>
	</div>

	{#if error}
		<div class="rounded-md border border-destructive bg-destructive/10 p-4">
			<p class="text-sm text-destructive">{error}</p>
		</div>
	{/if}

	<!-- Metrics Grid -->
	<div class="grid grid-cols-1 gap-4 md:grid-cols-4">
		{#if loading}
			{#each Array(8) as _}
				<Card>
					<CardContent class="flex gap-3 flex-col">
						<div class="flex items-center justify-between">
							<Skeleton class="h-4 w-24" />
							<Skeleton class="h-2 w-2 rounded-full" />
						</div>
						<Skeleton class="h-8 w-32" />
						<Skeleton class="h-16 w-full" />
						<Skeleton class="h-3 w-28" />
					</CardContent>
				</Card>
			{/each}
		{:else if dashboardData}
			{#each dashboardData.metrics as metric (metric.id)}
				<MetricCard {metric} />
			{/each}
		{/if}
	</div>
</div>
