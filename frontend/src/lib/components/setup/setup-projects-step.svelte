<script lang="ts">
	import { onMount } from 'svelte';
	import { ArrowRight } from '@lucide/svelte';
	import { Portal } from 'bits-ui';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { projectsState } from '$lib/state/projects.svelte';
	import { gotoHref } from '$lib/utils/navigation';
	import { getSetupMode, type SetupMode } from '$lib/utils/setup-storage';
	import type { SetupDraft } from '$lib/utils/setup-plan';
	import type { Framework } from '$lib/state/projects.svelte';
	import SetupModeTabs from './setup-mode-tabs.svelte';
	import SetupTokenPanel from './setup-token-panel.svelte';
	import SetupDraftReview from './setup-draft-review.svelte';
	import SetupNextSteps from './setup-next-steps.svelte';
	import SetupLiveProjects from './setup-live-projects.svelte';
	import ManualProjectsEditor from './manual-projects-editor.svelte';
	import PartyIcon from './party-icon.svelte';

	let {
		organizationId,
		redirectTo = null,
		initialFramework = 'opentelemetry'
	}: {
		organizationId: number;
		redirectTo?: string | null;
		initialFramework?: Framework;
	} = $props();

	let mode = $state<SetupMode>(getSetupMode());
	let draft = $state<SetupDraft | null>(null);
	let draftLoaded = $state(false);
	let initialProjectIds = $state<Set<string>>(new Set());
	let snapshotTaken = $state(false);
	let pollInFlight = false;
	let celebrating = $state(false);
	let celebrated = false;

	const orgProjects = $derived(
		projectsState.projects.filter((p) => p.organizationId === organizationId)
	);
	const newProjects = $derived(orgProjects.filter((p) => !initialProjectIds.has(p.id)));
	// Deployment credentials are only shown on this page, so the auto-redirect
	// must not run past them.
	const hasDeploymentSteps = $derived(
		draft?.status === 'approved' && draft.payload.projects.some((p) => p.deployment)
	);

	function destination(): string {
		if (redirectTo) return redirectTo;
		const first = newProjects[0] ?? orgProjects[0];
		return first ? `/?projectId=${first.id}` : '/';
	}

	function finish() {
		gotoHref(destination(), { replaceState: true });
	}

	// The first created projects trigger the celebration; it redirects to the
	// new project's dashboard unless deployment credentials still need showing.
	$effect(() => {
		if (celebrated || !snapshotTaken || newProjects.length === 0) return;
		celebrated = true;
		celebrating = true;
		const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		setTimeout(
			() => {
				celebrating = false;
				if (!hasDeploymentSteps) finish();
			},
			reduced ? 400 : 2000
		);
	});

	async function refresh() {
		if (pollInFlight) return;
		pollInFlight = true;
		try {
			const [draftResponse] = await Promise.all([
				api.get(`/setup/drafts?organizationId=${organizationId}`),
				projectsState.loadProjects()
			]);
			draft = draftResponse?.draft ?? null;
			draftLoaded = true;
		} catch {
		} finally {
			pollInFlight = false;
		}
	}

	onMount(() => {
		initialProjectIds = new Set(
			projectsState.projects.filter((p) => p.organizationId === organizationId).map((p) => p.id)
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
		{#if !draftLoaded}
			<p class="text-sm text-muted-foreground">Loading the latest setup proposal...</p>
		{:else if draft?.status === 'pending'}
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
			{#if newProjects.length === 0}
				<SetupTokenPanel {organizationId} autoMint={draft === null} />
			{/if}
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
			<SetupLiveProjects projects={orgProjects} {initialProjectIds} waitingText="" />
		{/if}
	{/if}

	{#if hasDeploymentSteps && newProjects.length > 0 && !celebrating}
		<div class="flex justify-end pt-2">
			<Button onclick={finish}>
				Continue to Dashboard
				<ArrowRight class="ml-2 h-4 w-4" />
			</Button>
		</div>
	{/if}
</div>

{#if celebrating}
	<Portal>
		<div class="celebration" aria-hidden="true">
			<span class="celebration-icon">
				<PartyIcon />
			</span>
		</div>
	</Portal>
{/if}

<style>
	.celebration {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: grid;
		place-items: center;
		pointer-events: none;
	}

	.celebration-icon {
		width: 20rem;
		height: 20rem;
		max-width: 80%;
		max-height: 80vw;
		padding: 50px;
		background: #fff;
		border-radius: 100%;
		filter: drop-shadow(0 12px 32px rgb(0 0 0 / 0.18));
		animation: setup-pop 2s cubic-bezier(0.34, 1.4, 0.64, 1) forwards;
	}

	@keyframes setup-pop {
		0% {
			opacity: 0;
			transform: scale(0.2);
		}
		15% {
			opacity: 1;
			transform: scale(1.12);
		}
		25% {
			transform: scale(1);
		}
		80% {
			opacity: 1;
			transform: scale(1);
		}
		100% {
			opacity: 0;
			transform: scale(1.3);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.celebration {
			display: none;
		}
	}
</style>
