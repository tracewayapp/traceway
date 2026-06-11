<script lang="ts">
	import bash from 'svelte-highlight/languages/bash';
	import plaintext from 'svelte-highlight/languages/plaintext';
	import { SKILL_INSTALL_COMMAND, getSetupPrompt } from '$lib/utils/ai-setup';
	import CopyableCodeBlock from './copyable-code-block.svelte';

	let { backendUrl, token }: { backendUrl: string; token: string } = $props();

	const setupPrompt = $derived(getSetupPrompt(backendUrl, token));
</script>

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
			Paste this prompt into your agent. Your instance URL and project token are already filled in.
		</p>
	</div>
	<div class="p-4">
		<CopyableCodeBlock code={setupPrompt} language={plaintext} wrap />
	</div>
</div>
