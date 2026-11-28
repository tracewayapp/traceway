<script lang="ts">
	import { resolve } from '$app/paths';
	import { projectsState } from '$lib/state/projects.svelte';
	import { sdkPlatform } from '$lib/components/sdk-config/sdk-config';
	import { recordingSequence } from './session-actions';
	let {
		reasons = [],
		detailed = false
	}: {
		reasons?: Array<{ triggerId: string; name: string; sequence: string; revision: string }>;
		/** Also spell out the steps next to each chip; a list only has room for the chip. */
		detailed?: boolean;
	} = $props();

	const platform = $derived(sdkPlatform(projectsState.currentProject?.framework));
	// A chip stands for a trigger revision. ClickHouse can return one twice until it merges, labelled before and
	// after the server renamed its steps, and a repeated key would throw while rendering.
	const key = (reason: { triggerId: string; revision: string }) =>
		`${reason.triggerId}:${reason.revision}`;
	const shown = $derived(
		reasons.filter((reason, i) => reasons.findIndex((other) => key(other) === key(reason)) === i)
	);

	function steps(reason: { sequence: string }): string {
		return recordingSequence(reason.sequence, platform);
	}

	// A chip says one thing: the trigger's name, or its steps when it was never given one.
	function label(reason: { triggerId: string; name: string; sequence: string }): string {
		return reason.name && reason.name !== reason.triggerId ? reason.name : steps(reason);
	}
</script>

<div class="flex flex-wrap items-center gap-x-2 gap-y-1.5" aria-label="Recording triggers">
	{#each shown as reason (key(reason))}
		<a
			class="inline-flex max-w-full items-center rounded-full border border-yellow-600 bg-yellow-600 px-2.5 py-0.5 text-xs font-semibold text-white hover:border-yellow-700 hover:bg-yellow-700"
			class:font-mono={label(reason) === steps(reason)}
			href={resolve(
				`/sessions?projectId=${encodeURIComponent(projectsState.currentProjectId ?? '')}&triggerId=${encodeURIComponent(reason.triggerId)}&preset=24h`
			)}
			onclick={(event) => event.stopPropagation()}
			title={`${steps(reason)}\nClick to show the sessions this trigger recorded`}
		>
			<span class="truncate">{label(reason)}</span>
		</a>
		{#if detailed && label(reason) !== steps(reason)}
			<span class="font-mono text-xs break-all text-muted-foreground">{steps(reason)}</span>
			<span class="basis-full"></span>
		{/if}
	{/each}
</div>
