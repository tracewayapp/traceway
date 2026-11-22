<script lang="ts">
	import { onMount } from 'svelte';
	import WidgetGrid from '$lib/components/dashboard/widget-grid.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { scheduleEmbedExpiry, type LoadEmbedWidget } from '$lib/utils/dashboard-embed';
	import type { DashboardWidget, MetricQueryResponse } from '$lib/types/dashboard';

	type EmbedDashboard = {
		name: string;
		from: string;
		to: string;
		expiresInSeconds: number;
		widgetToken?: string;
		widgets: DashboardWidget[];
	};

	// The dashboard and the reader holding its token change together, so a renewed
	// link replaces what is on screen in one step instead of blanking the frame.
	let view = $state.raw<{ dashboard: EmbedDashboard; load: LoadEmbedWidget } | null>(null);
	let error = $state('');
	let href = $state('');
	onMount(() => {
		href = window.location.href;
	});

	$effect(() => {
		// Assigning a renewed iframe src can be a same-document fragment navigation.
		if (!href) return;
		const url = new URL(href);
		const uuid = url.searchParams.get('dashboard');
		const token = new URLSearchParams(url.hash.slice(1)).get('token');
		error = '';
		if (!uuid || !token) {
			view = null;
			error = 'This dashboard link is incomplete.';
			return;
		}
		const endpoint = `/api/embedded-dashboards/${encodeURIComponent(uuid)}`;
		const controller = new AbortController();

		function fail(message: string) {
			if (controller.signal.aborted) return;
			view = null;
			error = message;
		}
		async function read<T>(path: string, bearer = token): Promise<T> {
			const response = await fetch(endpoint + path, {
				headers: { Authorization: `Bearer ${bearer}` },
				credentials: 'omit',
				cache: 'no-store',
				signal: controller.signal
			});
			if (!response.ok) {
				if (response.status === 401) fail('This dashboard link is invalid or has expired.');
				throw Object.assign(new Error('Could not load the shared dashboard.'), {
					status: response.status
				});
			}
			return response.json();
		}
		read<EmbedDashboard>('')
			.then((dashboard) => {
				if (controller.signal.aborted) return;
				error = '';
				view = {
					dashboard,
					load: (id) =>
						read<MetricQueryResponse>(
							`/widgets/${encodeURIComponent(id)}`,
							dashboard.widgetToken ?? token
						)
				};
			})
			.catch((e: { status?: number }) => {
				if (e.status === 401) return;
				fail(
					e.status === 404
						? 'This dashboard is no longer shared.'
						: 'Could not load the shared dashboard.'
				);
			});
		return () => controller.abort();
	});
	// Keep the displayed view's deadline even while its replacement is loading.
	$effect(() => {
		if (!view) return;
		return scheduleEmbedExpiry(view.dashboard.expiresInSeconds, () => {
			view = null;
			error = 'This dashboard link has expired.';
		});
	});
</script>

<svelte:window onhashchange={() => (href = window.location.href)} />

<svelte:head><title>{view?.dashboard.name ?? 'Shared Dashboard'} · Traceway</title></svelte:head>

<div class="min-h-full bg-background p-4 text-foreground">
	{#if error}
		<p role="alert" class="text-sm text-muted-foreground">{error}</p>
	{:else if view}
		{@const dashboard = view.dashboard}
		<header class="mb-4">
			<h1 class="text-xl font-semibold">{dashboard.name}</h1>
			<p class="text-sm text-muted-foreground">
				{new Date(dashboard.from).toLocaleString()} – {new Date(dashboard.to).toLocaleString()}
			</p>
		</header>
		{#if dashboard.widgets?.length}
			{#key view}
				<WidgetGrid
					widgets={dashboard.widgets.map((widget, position) => ({ ...widget, position }))}
					fromDateUTC={dashboard.from}
					toDateUTC={dashboard.to}
					timeDomain={[new Date(dashboard.from), new Date(dashboard.to)]}
					loadEmbedWidget={view.load}
				/>
			{/key}
		{:else}
			<p class="text-sm text-muted-foreground">This dashboard has no widgets.</p>
		{/if}
	{:else}
		<LoadingCircle size="md" />
	{/if}
</div>
