<script lang="ts">
	import { onMount } from 'svelte';
	import WidgetGrid from '$lib/components/dashboard/widget-grid.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import type { DashboardWidget, MetricQueryResponse } from '$lib/types/dashboard';

	type EmbedDashboard = {
		name: string;
		from: string;
		to: string;
		expiresAt: string;
		widgets: DashboardWidget[];
	};

	let dashboard = $state<EmbedDashboard | null>(null);
	let error = $state('');
	let href = $state('');
	onMount(() => {
		href = window.location.href;
	});
	let loadEmbedWidget = $state<(id: string | number) => Promise<MetricQueryResponse>>();

	$effect(() => {
		// Assigning a renewed iframe src can be a same-document fragment navigation.
		if (!href) return;
		const url = new URL(href);
		const uuid = url.searchParams.get('dashboard');
		const token = new URLSearchParams(url.hash.slice(1)).get('token');
		dashboard = null;
		error = '';
		if (!uuid || !token) {
			error = 'This dashboard link is incomplete.';
			return;
		}
		const endpoint = `/api/embedded-dashboards/${encodeURIComponent(uuid)}`;
		const controller = new AbortController();
		let timer: ReturnType<typeof setTimeout>;
		async function read<T>(path: string): Promise<T> {
			const response = await fetch(endpoint + path, {
				headers: { Authorization: `Bearer ${token}` },
				credentials: 'omit',
				cache: 'no-store',
				signal: controller.signal
			});
			if (!response.ok) {
				if (response.status === 401 && !controller.signal.aborted)
					error = 'This dashboard link is invalid or has expired.';
				throw new Error('Could not load the shared dashboard.');
			}
			return response.json();
		}
		loadEmbedWidget = (id) => read<MetricQueryResponse>(`/widgets/${encodeURIComponent(id)}`);
		read<EmbedDashboard>('')
			.then((result) => {
				if (controller.signal.aborted) return;
				dashboard = result;
				timer = setTimeout(
					() => {
						dashboard = null;
						error = 'This dashboard link has expired.';
					},
					Math.max(0, Date.parse(result.expiresAt) - Date.now())
				);
			})
			.catch(() => {
				if (!error && !controller.signal.aborted) error = 'Could not load the shared dashboard.';
			});
		return () => {
			controller.abort();
			clearTimeout(timer);
		};
	});
</script>

<svelte:window onhashchange={() => (href = window.location.href)} />

<svelte:head><title>{dashboard?.name ?? 'Shared Dashboard'} · Traceway</title></svelte:head>

<div class="min-h-full bg-background p-4 text-foreground">
	{#if error}
		<p role="alert" class="text-sm text-muted-foreground">{error}</p>
	{:else if dashboard && loadEmbedWidget}
		<header class="mb-4">
			<h1 class="text-xl font-semibold">{dashboard.name}</h1>
			<p class="text-sm text-muted-foreground">
				{new Date(dashboard.from).toLocaleString()} – {new Date(dashboard.to).toLocaleString()}
			</p>
		</header>
		{#if dashboard.widgets?.length}
			<WidgetGrid
				widgets={dashboard.widgets.map((widget, position) => ({ ...widget, position }))}
				fromDateUTC={dashboard.from}
				toDateUTC={dashboard.to}
				timeDomain={[new Date(dashboard.from), new Date(dashboard.to)]}
				{loadEmbedWidget}
			/>
		{:else}
			<p class="text-sm text-muted-foreground">This dashboard has no widgets.</p>
		{/if}
	{:else}
		<LoadingCircle size="md" />
	{/if}
</div>
