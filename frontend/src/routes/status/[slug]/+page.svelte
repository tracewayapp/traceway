<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	interface StatusDay {
		bucket: string;
		total: number;
		up: number;
		missed: number;
	}

	interface StatusIncident {
		startedAt: string;
		resolvedAt: string | null;
	}

	interface StatusCheck {
		name: string;
		checkType: string;
		status: 'up' | 'down' | 'unknown';
		uptimePct: number | null;
		days: StatusDay[];
		incidents: StatusIncident[];
	}

	interface StatusView {
		name: string;
		description: string;
		hasLogo: boolean;
		checks: StatusCheck[];
		generatedAt: string;
	}

	let { data }: { data: { slug: string } } = $props();

	type LoadState = 'loading' | 'ready' | 'notFound' | 'error';
	let loadState = $state<LoadState>('loading');
	let view = $state<StatusView | null>(null);
	let refreshTimer: ReturnType<typeof setInterval> | null = null;

	const DAYS = 90;

	type DayCell = {
		date: Date;
		state: 'operational' | 'degraded' | 'down' | 'empty';
		uptimePct: number | null;
	};

	// Deliberately a raw fetch: the api client's 401 handling would bounce
	// anonymous visitors to /login, and this page must work logged out.
	async function loadStatus() {
		try {
			const response = await fetch(`/api/status/${encodeURIComponent(data.slug)}`);
			if (response.status === 404) {
				loadState = 'notFound';
				return;
			}
			if (!response.ok) {
				throw new Error(`status ${response.status}`);
			}
			view = await response.json();
			loadState = 'ready';
		} catch {
			if (loadState !== 'ready') {
				loadState = 'error';
			}
		}
	}

	const allUp = $derived(view !== null && view.checks.every((c) => c.status !== 'down'));

	function dayKey(date: Date): string {
		return `${date.getUTCFullYear()}-${date.getUTCMonth()}-${date.getUTCDate()}`;
	}

	// A fixed 90-cell grid: days without data render as empty cells instead of
	// the bar silently shrinking.
	function buildCells(check: StatusCheck): DayCell[] {
		const byDay = new Map<string, StatusDay>();
		for (const day of check.days) {
			byDay.set(dayKey(new Date(day.bucket)), day);
		}
		const cells: DayCell[] = [];
		const now = new Date();
		for (let i = DAYS - 1; i >= 0; i--) {
			const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
			date.setUTCDate(date.getUTCDate() - i);
			const day = byDay.get(dayKey(date));
			const measured = day ? day.total - day.missed : 0;
			if (!day || measured === 0) {
				cells.push({ date, state: 'empty', uptimePct: null });
				continue;
			}
			const pct = (day.up / measured) * 100;
			cells.push({
				date,
				state: pct >= 99.5 ? 'operational' : day.up === 0 ? 'down' : 'degraded',
				uptimePct: pct
			});
		}
		return cells;
	}

	const cellColors: Record<DayCell['state'], string> = {
		operational: '#22c55e',
		degraded: '#f5a623',
		down: '#e5484d',
		empty: '#e4e7ec'
	};

	const stateDescriptions: Record<DayCell['state'], string> = {
		operational: 'Operational',
		degraded: 'Partial outage',
		down: 'Major outage',
		empty: 'No data'
	};

	function cellTitle(cell: DayCell): string {
		const date = cell.date.toLocaleDateString(undefined, {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
		if (cell.state === 'empty') return `${date} — no data`;
		return `${date} — ${stateDescriptions[cell.state]} (${cell.uptimePct?.toFixed(2)}% uptime)`;
	}

	function checkStatusLabel(check: StatusCheck): { label: string; color: string } {
		if (check.status === 'up') return { label: 'Operational', color: '#16a34a' };
		if (check.status === 'down') return { label: 'Major outage', color: '#e5484d' };
		return { label: 'No recent data', color: '#98a2b3' };
	}

	function formatIncident(incident: StatusIncident): string {
		const started = new Date(incident.startedAt).toLocaleString(undefined, {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
		if (!incident.resolvedAt) return `${started} — ongoing`;
		const minutes = Math.max(
			1,
			Math.round(
				(new Date(incident.resolvedAt).getTime() - new Date(incident.startedAt).getTime()) / 60000
			)
		);
		return `${started} — resolved after ${minutes} min`;
	}

	// The status page is a standalone light design; anonymous visitors default
	// to the app's dark theme class, which would restyle headings underneath
	// the scoped styles. Force light here and restore on leave.
	let hadDarkClass = false;
	onMount(() => {
		hadDarkClass = document.documentElement.classList.contains('dark');
		document.documentElement.classList.remove('dark');
		document.documentElement.style.colorScheme = 'light';
		loadStatus();
		refreshTimer = setInterval(loadStatus, 60000);
	});

	onDestroy(() => {
		if (refreshTimer) clearInterval(refreshTimer);
		if (hadDarkClass) {
			document.documentElement.classList.add('dark');
			document.documentElement.style.colorScheme = 'dark';
		}
	});
</script>

<svelte:head>
	<title>{view ? `${view.name} status` : 'Status'}</title>
</svelte:head>

<!-- Deliberately a standalone light design, independent of the app theme. -->
<div class="status-page">
	<div class="status-container">
		{#if loadState === 'loading'}
			<div class="status-center">
				<div class="status-spinner"></div>
			</div>
		{:else if loadState === 'notFound'}
			<div class="status-center">
				<h1>Status page not found</h1>
				<p>This status page does not exist or is not public.</p>
			</div>
		{:else if loadState === 'error'}
			<div class="status-center">
				<h1>Something went wrong</h1>
				<p>Could not load the status page. Try again shortly.</p>
			</div>
		{:else if view}
			<header class="status-header">
				<div class="status-brand">
					{#if view.hasLogo}
						<img src={`/api/status/${encodeURIComponent(data.slug)}/logo`} alt="" />
					{/if}
					<div>
						<h1>{view.name}</h1>
						{#if view.description}
							<p>{view.description}</p>
						{/if}
					</div>
				</div>
			</header>

			<div class="status-banner" class:degraded={!allUp}>
				{allUp ? 'All systems operational' : 'Some systems are experiencing issues'}
			</div>

			<div class="status-cards">
				{#each view.checks as check}
					{@const cells = buildCells(check)}
					{@const chip = checkStatusLabel(check)}
					<section class="status-card">
						<div class="status-card-head">
							<span class="status-card-name">{check.name}</span>
							<span class="status-chip" style="color: {chip.color}">
								<span class="status-dot" style="background: {chip.color}"></span>
								{chip.label}
							</span>
						</div>
						<div class="status-uptime-row">
							<span
								>{check.uptimePct === null
									? 'No data yet'
									: `${check.uptimePct.toFixed(2)}% uptime in the last 90 days`}</span
							>
						</div>
						<div class="status-bars">
							{#each cells as cell, i}
								<span
									class="status-bar {i < 30 ? 'band-oldest' : i < 60 ? 'band-mid' : ''}"
									style="background: {cellColors[cell.state]}"
									title={cellTitle(cell)}
								></span>
							{/each}
						</div>
						<div class="status-bar-labels">
							<span class="label-90">90 days ago</span>
							<span class="label-60">60 days ago</span>
							<span class="label-30">30 days ago</span>
							<span>Today</span>
						</div>
						{#if check.incidents.length > 0}
							<details class="status-incidents">
								<summary>
									{check.incidents.length} incident{check.incidents.length === 1 ? '' : 's'} in the last
									90 days
								</summary>
								<ul>
									{#each check.incidents as incident}
										<li>{formatIncident(incident)}</li>
									{/each}
								</ul>
							</details>
						{/if}
					</section>
				{/each}
			</div>

			<div class="status-legend">
				<span><i style="background: #22c55e"></i> Operational</span>
				<span><i style="background: #f5a623"></i> Partial outage</span>
				<span><i style="background: #e5484d"></i> Major outage</span>
				<span><i style="background: #e4e7ec"></i> No data</span>
			</div>

			<footer class="status-footer">
				Updated {new Date(view.generatedAt).toLocaleTimeString()} · Powered by Traceway
			</footer>
		{/if}
	</div>
</div>

<style>
	.status-page {
		min-height: 100vh;
		background: #f7f8fa;
		color: #101828;
		font-family:
			-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
	}

	.status-container {
		max-width: 880px;
		margin: 0 auto;
		padding: 56px 24px 40px;
	}

	.status-center {
		padding: 120px 0;
		text-align: center;
	}

	.status-center h1 {
		font-size: 24px;
		font-weight: 600;
		margin: 0 0 8px;
		color: #101828;
	}

	.status-center p {
		color: #667085;
		margin: 0;
	}

	.status-spinner {
		width: 28px;
		height: 28px;
		margin: 0 auto;
		border: 3px solid #e4e7ec;
		border-top-color: #16a34a;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.status-header {
		margin-bottom: 20px;
	}

	.status-brand {
		display: flex;
		align-items: center;
		gap: 16px;
	}

	.status-brand img {
		width: 48px;
		height: 48px;
		border-radius: 10px;
		object-fit: contain;
		background: #fff;
		border: 1px solid #e4e7ec;
		padding: 4px;
	}

	.status-brand h1 {
		font-size: 26px;
		font-weight: 700;
		margin: 0;
		letter-spacing: -0.02em;
		color: #101828;
	}

	.status-brand p {
		margin: 2px 0 0;
		color: #667085;
		font-size: 14px;
	}

	.status-banner {
		background: #16a34a;
		color: #fff;
		font-size: 16px;
		font-weight: 600;
		padding: 16px 20px;
		border-radius: 10px;
		margin-bottom: 24px;
	}

	.status-banner.degraded {
		background: #e5484d;
	}

	.status-cards {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.status-card {
		background: #fff;
		border: 1px solid #e4e7ec;
		border-radius: 12px;
		padding: 20px;
		box-shadow: 0 1px 2px rgba(16, 24, 40, 0.04);
	}

	.status-card-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.status-card-name {
		font-size: 15px;
		font-weight: 600;
	}

	.status-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 600;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.status-uptime-row {
		margin-top: 2px;
		font-size: 12px;
		color: #667085;
	}

	.status-bars {
		display: flex;
		gap: 3px;
		margin-top: 12px;
	}

	.status-bar {
		flex: 1;
		height: 34px;
		min-width: 3px;
		border-radius: 3px;
		transition: transform 0.1s ease;
	}

	.status-bar:hover {
		transform: scaleY(1.15);
	}

	.status-bar-labels {
		display: flex;
		justify-content: space-between;
		margin-top: 6px;
		font-size: 11px;
		color: #98a2b3;
	}

	.label-60,
	.label-30 {
		display: none;
	}

	/* Narrow screens can't fit 90 bars (3px bar + 3px gap each), so show a
	   shorter recent window instead of overflowing the viewport. */
	@media (max-width: 760px) {
		.status-bar.band-oldest {
			display: none;
		}

		.label-90 {
			display: none;
		}

		.label-60 {
			display: inline;
		}
	}

	@media (max-width: 520px) {
		.status-bar.band-mid {
			display: none;
		}

		.label-60 {
			display: none;
		}

		.label-30 {
			display: inline;
		}
	}

	@media (max-width: 640px) {
		.status-container {
			padding: 32px 16px 32px;
		}

		.status-brand h1 {
			font-size: 22px;
		}

		.status-banner {
			font-size: 14px;
			padding: 14px 16px;
		}

		.status-card {
			padding: 16px;
		}

		.status-card-head {
			flex-wrap: wrap;
		}
	}

	.status-incidents {
		margin-top: 12px;
		font-size: 12px;
		color: #667085;
	}

	.status-incidents summary {
		cursor: pointer;
	}

	.status-incidents ul {
		margin: 8px 0 0;
		padding-left: 18px;
	}

	.status-incidents li {
		margin-top: 4px;
	}

	.status-legend {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
		justify-content: center;
		margin-top: 28px;
		font-size: 12px;
		color: #667085;
	}

	.status-legend i {
		display: inline-block;
		width: 10px;
		height: 10px;
		border-radius: 2px;
		margin-right: 5px;
	}

	.status-footer {
		margin-top: 28px;
		text-align: center;
		font-size: 12px;
		color: #98a2b3;
	}
</style>
