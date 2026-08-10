<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Check, CircleCheck } from '@lucide/svelte';
	import { themeState } from '$lib/state/theme.svelte';

	interface AckPage {
		subject: string;
		body: string;
		severity: string;
		urgency: '' | 'high' | 'low';
		status: 'open' | 'acknowledged';
		ruleName: string;
		projectName: string;
		eventCount: number;
		lastEventAt: string;
		createdAt: string;
		acknowledgedAt: string | null;
		acknowledgedByName: string;
	}

	let { data }: { data: { token: string } } = $props();

	type Status = 'loading' | 'ready' | 'acknowledging' | 'justAcknowledged' | 'invalid' | 'error';

	let status = $state<Status>('loading');
	let error = $state('');
	let info = $state<AckPage | null>(null);

	function formatTime(value: string | null): string {
		if (!value) return '';
		return new Date(value).toLocaleString(undefined, {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	async function load() {
		status = 'loading';
		error = '';
		try {
			const res = await fetch(`/api/ack/${encodeURIComponent(data.token)}`);
			if (res.status === 404) {
				status = 'invalid';
				return;
			}
			if (!res.ok) {
				status = 'error';
				error = 'We could not load this page. Please try again.';
				return;
			}
			info = await res.json();
			status = 'ready';
		} catch {
			status = 'error';
			error = 'We could not reach the server. Please try again.';
		}
	}

	async function acknowledge() {
		status = 'acknowledging';
		error = '';
		try {
			const res = await fetch(`/api/ack/${encodeURIComponent(data.token)}`, { method: 'POST' });
			if (res.status === 404) {
				status = 'invalid';
				return;
			}
			if (!res.ok) {
				status = 'ready';
				error = 'We could not acknowledge the page. Please try again.';
				return;
			}
			status = 'justAcknowledged';
		} catch {
			status = 'ready';
			error = 'We could not reach the server. Please try again.';
		}
	}

	onMount(() => {
		load();
	});
</script>

<div class="flex min-h-screen w-full items-center justify-center px-4 py-8">
	<Card class="w-full max-w-[480px]">
		<CardHeader>
			<CardTitle class="text-2xl">
				<div class="flex flex-row items-center justify-center gap-2">
					{#if themeState.isDark}
						<img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-8 w-auto" />
					{:else}
						<img src="/traceway-logo.png" alt="Traceway Logo" class="h-8 w-auto" />
					{/if}
				</div>
			</CardTitle>
			<CardDescription class="text-center">On-call page</CardDescription>
		</CardHeader>
		<CardContent>
			{#if status === 'loading'}
				<div class="flex items-center justify-center py-8">
					<div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary"></div>
				</div>
			{:else if status === 'invalid'}
				<WarningCallout variant="destructive" title="Link no longer valid">
					This link is no longer valid. The page may have been resolved.
				</WarningCallout>
			{:else if status === 'justAcknowledged'}
				<div class="flex flex-col items-center justify-center gap-4 py-8">
					<div class="rounded-full bg-green-100 p-3">
						<Check class="h-8 w-8 text-green-600" />
					</div>
					<p class="text-center text-lg font-medium">Acknowledged</p>
					<p class="text-center text-muted-foreground">The on-call escalation has stopped.</p>
				</div>
			{:else if (status === 'ready' || status === 'acknowledging') && info}
				<div class="space-y-4">
					<ErrorAlert {error} />

					<div class="space-y-2">
						<h2 class="text-lg font-semibold">{info.subject || 'Page'}</h2>
						<div class="flex flex-wrap items-center gap-2">
							{#if info.severity === 'critical'}
								<Badge variant="destructive">Critical</Badge>
							{:else if info.severity === 'warning'}
								<Badge class="bg-amber-500 text-white">Warning</Badge>
							{:else if info.severity === 'info'}
								<Badge variant="secondary">Info</Badge>
							{/if}
							{#if info.urgency === 'high'}
								<Badge class="bg-rose-700 text-white">High urgency</Badge>
							{:else if info.urgency === 'low'}
								<Badge variant="secondary">Low urgency</Badge>
							{/if}
						</div>
					</div>

					<div class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
						<div>
							<div class="text-xs text-muted-foreground">Project</div>
							<div>{info.projectName || '—'}</div>
						</div>
						<div>
							<div class="text-xs text-muted-foreground">Rule</div>
							<div>{info.ruleName || '—'}</div>
						</div>
						<div>
							<div class="text-xs text-muted-foreground">Events</div>
							<div>{info.eventCount}</div>
						</div>
						<div>
							<div class="text-xs text-muted-foreground">Opened</div>
							<div>{formatTime(info.createdAt)}</div>
						</div>
					</div>

					{#if info.body}
						<p class="line-clamp-6 text-sm whitespace-pre-wrap text-muted-foreground">
							{info.body}
						</p>
					{/if}

					{#if info.status === 'acknowledged'}
						<div
							class="flex items-center justify-center gap-2 rounded-md bg-muted py-3 text-sm text-muted-foreground"
						>
							<CircleCheck class="h-4 w-4 text-green-600" />
							Acknowledged{info.acknowledgedByName ? ` by ${info.acknowledgedByName}` : ''}
							{info.acknowledgedAt ? ` at ${formatTime(info.acknowledgedAt)}` : ''}
						</div>
					{:else}
						<Button
							class="w-full"
							size="lg"
							disabled={status === 'acknowledging'}
							onclick={acknowledge}
						>
							<Check class="mr-2 h-5 w-5" />
							{status === 'acknowledging' ? 'Acknowledging...' : 'Acknowledge'}
						</Button>
					{/if}
				</div>
			{:else}
				<ErrorAlert title="Something went wrong" error={error || 'Please try again.'} />
			{/if}
		</CardContent>
	</Card>
</div>
