<script lang="ts">
	import { onMount } from 'svelte';
	import bash from 'svelte-highlight/languages/bash';
	import { RefreshCw } from '@lucide/svelte';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { SKILL_INSTALL_COMMAND, getSetupTokenPromptParts } from '$lib/utils/ai-setup';
	import type { SetupTokenResponse } from '$lib/utils/setup-plan';
	import CopyableCodeBlock from './copyable-code-block.svelte';
	import CopyablePrompt from './copyable-prompt.svelte';

	let { organizationId, autoMint = true }: { organizationId: number; autoMint?: boolean } =
		$props();

	let minting = $state(false);
	let mintError = $state('');
	let setupToken = $state<SetupTokenResponse | null>(null);
	let now = $state(Date.now());

	const minutesLeft = $derived.by(() => {
		if (!setupToken) return 0;
		return Math.max(0, Math.ceil((new Date(setupToken.expiresAt).getTime() - now) / 60000));
	});
	const expired = $derived(setupToken !== null && minutesLeft <= 0);
	const timeLeftLabel = $derived(
		minutesLeft >= 90
			? `${Math.floor(minutesLeft / 60)} h ${minutesLeft % 60} min`
			: `${minutesLeft} min`
	);
	const promptParts = $derived(
		setupToken ? getSetupTokenPromptParts(setupToken.backendUrl, setupToken.token) : []
	);

	async function mint() {
		minting = true;
		mintError = '';
		try {
			setupToken = await api.post('/setup-tokens', { organizationId });
		} catch (e) {
			mintError = e instanceof Error ? e.message : 'Failed to generate a setup token';
		} finally {
			minting = false;
		}
	}

	onMount(() => {
		if (autoMint) mint();
		const tick = setInterval(() => (now = Date.now()), 30000);
		return () => clearInterval(tick);
	});

	let lastOrgId: number | undefined;
	$effect(() => {
		if (lastOrgId === undefined) {
			lastOrgId = organizationId;
			return;
		}
		if (organizationId !== lastOrgId) {
			lastOrgId = organizationId;
			if (autoMint) mint();
		}
	});
</script>

<div class="space-y-4">
	<div class="rounded-md border bg-card">
		<div class="border-b px-4 py-3">
			<div class="flex items-center gap-3">
				<div
					class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
				>
					1
				</div>
				<h3 class="font-semibold">Install the Traceway Skill</h3>
			</div>
			<p class="mt-1 ml-9 text-sm text-muted-foreground">
				Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
				agent that supports agent skills.
			</p>
		</div>
		<div class="p-4">
			<CopyableCodeBlock code={SKILL_INSTALL_COMMAND} language={bash} />
		</div>
	</div>

	<div class="rounded-md border bg-card">
		<div class="border-b px-4 py-3">
			<div class="flex items-center gap-3">
				<div
					class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"
				>
					2
				</div>
				<h3 class="font-semibold">Run the Setup Prompt</h3>
			</div>
			<p class="mt-1 ml-9 text-sm text-muted-foreground">
				Paste this prompt into your agent. It analyzes your code and proposes the projects to
				create; the proposal appears here for your approval.
			</p>
		</div>
		<div class="space-y-3 p-4">
			{#if mintError}
				<ErrorAlert error={mintError} />
				<Button variant="outline" size="sm" onclick={mint} disabled={minting}>
					<RefreshCw class="mr-2 h-4 w-4" />
					Try Again
				</Button>
			{:else if minting}
				<p class="text-sm text-muted-foreground">Generating a setup token...</p>
			{:else if !setupToken}
				<p class="text-sm text-muted-foreground">
					Generate a replacement only after the current agent has received the proposal decision. A
					new token invalidates the previous one.
				</p>
				<Button variant="outline" size="sm" onclick={mint} disabled={minting}>
					<RefreshCw class="mr-2 h-4 w-4" />
					Generate New Token
				</Button>
			{:else if expired}
				<p class="text-sm text-muted-foreground">This setup token expired.</p>
				<Button variant="outline" size="sm" onclick={mint} disabled={minting}>
					<RefreshCw class="mr-2 h-4 w-4" />
					Generate New Token
				</Button>
			{:else}
				<CopyablePrompt parts={promptParts} />
				<p class="text-xs text-muted-foreground">
					The setup token lets your coding agent propose a setup plan for this organization for the
					next {timeLeftLabel}. It is not an ingest token, and nothing is created until you approve
					the proposal here.
				</p>
			{/if}
		</div>
	</div>
</div>
