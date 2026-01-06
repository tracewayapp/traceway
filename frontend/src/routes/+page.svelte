<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { RefreshCw } from 'lucide-svelte';
	import MetricCard from '$lib/components/dashboard/metric-card.svelte';
	import type { DashboardData, DashboardMetric } from '$lib/types/dashboard';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { api } from '$lib/api';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import { projectsState } from '$lib/state/projects.svelte';

	let dashboardData = $state<DashboardData | null>(null);
	let loading = $state(true);
	let error = $state('');
	let errorStatus = $state<number>(0);

	async function loadDashboard() {
		loading = true;
		error = '';
		errorStatus = 0;

		try {
			const response = await api.get('/dashboard', { projectId: projectsState.currentProjectId ?? undefined });

			// Transform the API response to match the DashboardData type
			// Convert timestamp strings to Date objects
			const metrics: DashboardMetric[] = response.metrics.map((m: any) => ({
				id: m.id,
				name: m.name,
				value: m.value,
				unit: m.unit,
				trend: m.trend.map((t: any) => ({
					timestamp: new Date(t.timestamp),
					value: t.value
				})),
				change24h: m.change24h,
				status: m.status
			}));

			dashboardData = {
				metrics,
				lastUpdated: new Date(response.lastUpdated)
			};
		} catch (e: any) {
			errorStatus = e.status || 0;
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

	{#if error && !loading}
		<ErrorDisplay
			status={errorStatus === 404 ? 404 : errorStatus === 400 ? 400 : errorStatus === 422 ? 422 : 400}
			title="Failed to Load Dashboard"
			description={error}
			onRetry={() => loadDashboard()}
		/>
	{/if}

	<!-- Metrics Grid -->
	{#if !error}
	<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
		{#if loading}
			{#each Array(6) as _}
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
	{/if}
</div>
