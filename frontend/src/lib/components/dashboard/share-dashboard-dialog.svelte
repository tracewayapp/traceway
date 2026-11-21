<script lang="ts">
	import { Check, Copy, Link2 } from '@lucide/svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import { getErrorMessage } from '$lib/utils/errors';
	import { scheduleEmbedExpiry } from '$lib/utils/dashboard-embed';
	import { REPLAY_BLOCK_CLASS } from '$lib/utils/self-monitoring';
	import { api } from '$lib/api';

	let {
		dashboardId,
		projectId,
		tagFilters,
		onClose
	}: {
		dashboardId: string;
		projectId: string;
		tagFilters: Record<string, string>;
		onClose: () => void;
	} = $props();

	const timeframes: Record<string, string> = {
		'30m': 'Last 30 minutes',
		'6h': 'Last 6 hours',
		'24h': 'Last 24 hours',
		'42h': 'Last 42 hours',
		'7d': 'Last 7 days',
		'30d': 'Last 30 days'
	};
	let days = $state<number | undefined>(1);
	let timeframe = $state('42h');
	let link = $state.raw<{ url: string; expiresAt: string; expiresInSeconds: number } | null>(null);
	let error = $state('');
	let loading = $state(false);
	let copied = $state(false);
	const endpoint = $derived(`/dashboards/${dashboardId}/embed-token`);

	function reset() {
		link = null;
		copied = false;
		error = '';
	}

	$effect(() => {
		if (!link) return;
		return scheduleEmbedExpiry(link.expiresInSeconds, () => {
			reset();
			error = 'This link has expired. Generate a new link to share.';
		});
	});

	async function generate() {
		reset();
		loading = true;
		try {
			const result: { embedPath: string; expiresAt: string; expiresInSeconds: number } =
				await api.post(
					`${endpoint}?timeframe=${timeframe}`,
					{ tagFilters, expiresInDays: days || 0 },
					{ projectId }
				);
			link = {
				url: new URL(result.embedPath, window.location.origin).href,
				expiresAt: result.expiresAt,
				expiresInSeconds: result.expiresInSeconds
			};
		} catch (e) {
			error = getErrorMessage(e) || 'Could not generate a dashboard link.';
		} finally {
			loading = false;
		}
	}

	async function copyLink() {
		if (!link) return;
		try {
			await navigator.clipboard.writeText(link.url);
			copied = true;
		} catch {
			error = 'Select the link and copy it manually.';
		}
	}
</script>

<AlertDialog.Root
	open
	onOpenChange={(open) => {
		if (!open) onClose();
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Share Dashboard</AlertDialog.Title>
			<AlertDialog.Description
				>Anyone with the link can view this dashboard and its selected filters until the link
				expires.</AlertDialog.Description
			>
		</AlertDialog.Header>
		<ErrorAlert {error} />
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<FormField label="Share for" for="share-days">
				<div class="relative">
					<Input
						id="share-days"
						type="number"
						min={1}
						step={1}
						class="pr-16"
						bind:value={days}
						oninput={reset}
						disabled={loading}
						aria-describedby="share-days-unit"
					/>
					<span
						id="share-days-unit"
						class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted-foreground"
						>{days === 1 ? 'day' : 'days'}</span
					>
				</div>
			</FormField>
			<FormField label="Timeframe" for="share-timeframe">
				<Select.Root
					type="single"
					value={timeframe}
					disabled={loading}
					onValueChange={(value) => {
						if (value) {
							timeframe = value;
							reset();
						}
					}}
				>
					<Select.Trigger id="share-timeframe" class="w-full"
						>{timeframes[timeframe]}</Select.Trigger
					>
					<Select.Content>
						{#each Object.entries(timeframes) as [value, label] (value)}
							<Select.Item {value} {label}>{label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</FormField>
		</div>
		<p class="text-sm text-muted-foreground">The timeframe ends when the viewer opens the link.</p>
		{#if link}
			<div class={REPLAY_BLOCK_CLASS}>
				<FormField label="Share link" for="dashboard-embed-url">
					<Input
						id="dashboard-embed-url"
						readonly
						value={link.url}
						onclick={(e) => e.currentTarget.select()}
					/>
				</FormField>
				<p class="mt-2 text-sm text-muted-foreground">
					Expires at <time datetime={link.expiresAt}
						>{new Date(link.expiresAt).toLocaleString(undefined, {
							dateStyle: 'medium',
							timeStyle: 'long'
						})}</time
					>
				</p>
			</div>
		{/if}
		<div class="space-y-2 border-t pt-4 text-sm text-muted-foreground">
			<p>
				To embed this dashboard in your application, request a link from your backend.
				<a
					href="https://docs.tracewayapp.com/learn/dashboards#share-and-embed-a-dashboard"
					target="_blank"
					rel="noopener noreferrer"
					class="underline hover:text-foreground">Embedding guide</a
				>
			</p>
			<code class="block rounded bg-muted px-2 py-1.5 font-mono text-xs break-all select-all"
				>POST /api{endpoint}?projectId={projectId}&amp;timeframe={timeframe}</code
			>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={onClose}>Close</Button>
			{#if link}
				<Button onclick={copyLink}
					>{#if copied}<Check />Copied{:else}<Copy />Copy Link{/if}</Button
				>
			{:else}
				<Button onclick={generate} disabled={loading}
					>{#if loading}<LoadingCircle size="sm" />Generating…{:else}<Link2 />Generate Link{/if}</Button
				>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
