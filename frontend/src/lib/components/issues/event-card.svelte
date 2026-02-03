<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { ArrowRight } from 'lucide-svelte';
	import type { ExceptionOccurrence, LinkedTrace } from '$lib/types/exceptions';
	import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { ContextGrid } from '$lib/components/ui/context-grid';
	import { LabelValue } from '../ui/label-value';

	interface Props {
		occurrence: ExceptionOccurrence;
		linkedTrace: LinkedTrace | null;
		title?: string;
		description?: string;
		timezone?: string;
	}

	let {
		occurrence,
		linkedTrace,
		title = 'Event',
		description = 'Details for this specific occurrence',
		timezone
	}: Props = $props();

	const tz = $derived(timezone ?? getTimezone());
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-6">
		<!-- Event Details -->
		<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Recorded At</p>
				<p class="font-mono text-sm">{formatDateTime(occurrence.recordedAt, { timezone: tz })}</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Server</p>
				<p class="font-mono text-sm">{occurrence.serverName || '-'}</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Version</p>
				<p class="font-mono text-sm">{occurrence.appVersion || '-'}</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Trace</p>
				<p class="font-mono text-sm">{occurrence.traceId || '-'}</p>
			</div>
		</div>

		<!-- Context -->
		{#if occurrence.attributes && Object.keys(occurrence.attributes).length > 0}
			<hr class="border-border" />
			<div>
				<p class="mb-3 text-sm font-medium">Attributes</p>
				<ContextGrid attributes={occurrence.attributes} />
			</div>
		{/if}

		<!-- Related Trace -->
		{#if linkedTrace}
			<hr class="border-border" />
			<div>
				<p class="mb-3 text-sm font-medium">
					{linkedTrace.traceType === 'task' ? 'Related Task' : 'Related Endpoint'}
				</p>
				<div class="mb-4 grid grid-cols-2 gap-4 md:grid-cols-4">
					<LabelValue
						label={linkedTrace.traceType === 'task' ? 'Task' : 'Endpoint'}
						value={linkedTrace.endpoint}
						mono
					/>
					{#if linkedTrace.traceType !== 'task'}
						<LabelValue
							label="Status"
							value={linkedTrace.statusCode}
							mono
							large
							valueClass={getStatusColor(linkedTrace.statusCode)}
						/>
					{/if}
					<LabelValue label="Duration" value={formatDuration(linkedTrace.duration)} mono large />
					<LabelValue
						label="Recorded At"
						value={formatDateTime(linkedTrace.recordedAt, { timezone })}
						mono
					/>
				</div>
				{#if linkedTrace.traceType === 'task'}
					<Button
						variant="outline"
						size="sm"
						onclick={() =>
							goto(
								`/tasks/${encodeURIComponent(linkedTrace.endpoint)}/${linkedTrace.id}?preset=24h`
							)}
					>
						View Task Details
						<ArrowRight class="ml-2 h-4 w-4" />
					</Button>
				{:else}
					<Button
						variant="outline"
						size="sm"
						onclick={() =>
							goto(
								`/endpoints/${encodeURIComponent(linkedTrace.endpoint)}/${linkedTrace.id}?preset=24h`
							)}
					>
						View Endpoint Details
						<ArrowRight class="ml-2 h-4 w-4" />
					</Button>
				{/if}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
