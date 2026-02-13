<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { ArrowRight } from 'lucide-svelte';
	import type { ExceptionOccurrence, LinkedTrace } from '$lib/types/exceptions';
	import { formatDuration, getStatusColor, formatDateTime } from '$lib/utils/formatters';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { AttributesGrid } from '$lib/components/ui/attributes-grid';
	import { LabelValue } from '../ui/label-value';
	import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import SessionReplay from './session-replay.svelte';

	interface Props {
		occurrence: ExceptionOccurrence;
		linkedTrace: LinkedTrace | null;
		sessionRecordingEvents?: unknown[] | null;
		title?: string;
		description?: string;
		timezone?: string;
	}

	let {
		occurrence,
		linkedTrace,
		sessionRecordingEvents = null,
		title = 'Event',
		description = 'Details for this specific occurrence',
		timezone
	}: Props = $props();

	const tz = $derived(timezone ?? getTimezone());
	const isFrontend = $derived(
		projectsState.currentProject?.framework
			? isFrontendFramework(projectsState.currentProject.framework)
			: false
	);
</script>

<!-- Session Replay -->
{#if sessionRecordingEvents && sessionRecordingEvents.length > 0}
	<Card.Root class="pb-0">
		<Card.Header class="gap-0">
			<Card.Title>Session Replay</Card.Title>
		</Card.Header>
		<Card.Content class="p-0">
			{#key sessionRecordingEvents}
			<SessionReplay events={sessionRecordingEvents as any} />
		{/key}
		</Card.Content>
	</Card.Root>
{/if}

<Card.Root>
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-6">
		<!-- Event Details -->
		<div class="grid grid-cols-2 gap-4 {isFrontend ? 'md:grid-cols-2' : 'md:grid-cols-4'}">
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Recorded At</p>
				<p class="font-mono text-sm">{formatDateTime(occurrence.recordedAt, { timezone: tz })}</p>
			</div>
			{#if !isFrontend}
				<div class="space-y-1">
					<p class="text-sm text-muted-foreground">Server</p>
					<p class="font-mono text-sm">{occurrence.serverName || '-'}</p>
				</div>
			{/if}
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Version</p>
				<p class="font-mono text-sm">{occurrence.appVersion || '-'}</p>
			</div>
			{#if !isFrontend}
				<div class="space-y-1">
					<p class="text-sm text-muted-foreground">Trace</p>
					<p class="font-mono text-sm">{occurrence.traceId || '-'}</p>
				</div>
			{/if}
		</div>

		<!-- AttributesGrid -->
		{#if occurrence.attributes && Object.keys(occurrence.attributes).length > 0}
			<hr class="border-border" />
			<div>
				<p class="mb-3 text-sm font-medium">Attributes</p>
				<AttributesGrid attributes={occurrence.attributes} />
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
