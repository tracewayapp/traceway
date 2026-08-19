<script lang="ts">
	import { onMount } from 'svelte';
	import { ArrowRight } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getSetupMode, type SetupMode } from '$lib/utils/setup-storage';
	import type { SetupDraft } from '$lib/utils/setup-plan';
	import type { Framework } from '$lib/state/projects.svelte';
	import SetupModeTabs from './setup-mode-tabs.svelte';
	import SetupTokenPanel from './setup-token-panel.svelte';
	import SetupDraftReview from './setup-draft-review.svelte';
	import SetupNextSteps from './setup-next-steps.svelte';
	import SetupLiveProjects from './setup-live-projects.svelte';
	import ManualProjectsEditor from './manual-projects-editor.svelte';

	let {
		organizationId,
		onDone,
		onSkip,
		continueLabel = 'Continue',
		initialFramework = 'opentelemetry'
	}: {
		organizationId: number;
		onDone: () => void;
		onSkip?: () => void;
		continueLabel?: string;
		initialFramework?: Framework;
	} = $props();

	let mode = $state<SetupMode>(getSetupMode());
	let draft = $state<SetupDraft | null>(null);
	let initialProjectIds = $state<Set<string>>(new Set());
	let snapshotTaken = $state(false);
	let pollInFlight = false;

	const orgProjects = $derived(
		projectsState.projects.filter((p) => p.organizationId === organizationId)
	);
	const newProjects = $derived(orgProjects.filter((p) => !initialProjectIds.has(p.id)));
	const canContinue = $derived(newProjects.length > 0 || draft?.status === 'approved');

	async function refresh() {
		if (pollInFlight) return;
		pollInFlight = true;
		try {
			const [draftResponse] = await Promise.all([
				api.get(`/setup/drafts?organizationId=${organizationId}`),
				projectsState.loadProjects()
			]);
			draft = draftResponse?.draft ?? null;
		} catch {
			// Transient polling errors are ignored; the next tick retries.
		} finally {
			pollInFlight = false;
		}
	}

	onMount(() => {
		initialProjectIds = new Set(
			projectsState.projects
				.filter((p) => p.organizationId === organizationId)
				.map((p) => p.id)
		);
		snapshotTaken = true;
		refresh();

		const interval = setInterval(() => {
			if (document.hidden) return;
			refresh();
		}, 3000);
		return () => clearInterval(interval);
	});
</script>

<div class="space-y-4">
	<SetupModeTabs {mode} onModeChange={(m) => (mode = m)} />

	{#if mode === 'ai'}
		{#if draft?.status === 'pending'}
			<SetupDraftReview {draft} onDecided={refresh} />
		{:else}
			{#if draft?.status === 'approved'}
				<SetupNextSteps {draft} />
			{/if}
			{#if draft?.status === 'rejected'}
				<p class="text-sm text-muted-foreground">
					You rejected the last proposal{draft.rejectReason ? ` (“${draft.rejectReason}”)` : ''}.
					Your agent can revise it and submit a new one with the same setup token.
				</p>
			{/if}
			<SetupTokenPanel {organizationId} />
		{/if}
		{#if snapshotTaken}
			<SetupLiveProjects
				projects={orgProjects}
				{initialProjectIds}
				waitingText="Your assistant's proposal will appear here for your approval, and created projects show up live."
			/>
		{/if}
	{:else}
		<ManualProjectsEditor {organizationId} {initialFramework} onCreated={() => refresh()} />
		{#if snapshotTaken && orgProjects.length > 0}
			<SetupLiveProjects
				projects={orgProjects}
				{initialProjectIds}
				waitingText=""
			/>
		{/if}
	{/if}

	<div class="flex items-center justify-between pt-2">
		{#if onSkip}
			<button
				type="button"
				class="text-sm text-muted-foreground underline-offset-4 hover:underline"
				onclick={onSkip}
			>
				Skip for now
			</button>
		{:else}
			<span></span>
		{/if}
		<div class="flex flex-col items-end gap-1">
			<Button onclick={onDone} disabled={!canContinue}>
				{continueLabel}
				<ArrowRight class="ml-2 h-4 w-4" />
			</Button>
			{#if !canContinue}
				<span class="text-xs text-muted-foreground">
					Waiting for your first project...
				</span>
			{/if}
		</div>
	</div>
</div>
