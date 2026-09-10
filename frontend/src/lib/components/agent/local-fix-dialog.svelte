<script lang="ts">
	import { browser } from '$app/environment';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import CopyableCodeBlock from '$lib/components/setup/copyable-code-block.svelte';
	import CopyablePrompt from '$lib/components/setup/copyable-prompt.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { underlineTabListClass, underlineTabTriggerClass } from '$lib/utils/tabs';
	import bash from 'svelte-highlight/languages/bash';
	import { ExternalLink, Terminal } from '@lucide/svelte';

	let { open = $bindable(false), hash }: { open: boolean; hash: string } = $props();
	let harness = $state('claude-code');
	const harnesses = [
		{ id: 'claude-code', name: 'Claude Code' },
		{ id: 'codex', name: 'Codex' },
		{ id: 'cursor', name: 'Cursor' },
		{ id: 'github-copilot', name: 'VS Code' },
		{ id: 'antigravity', name: 'Antigravity' }
	];
	const origin = $derived(browser ? window.location.origin : '');
	const installCommand = $derived(
		`npx skills add tracewayapp/traceway --skill traceway --agent ${harness}`
	);
	const loginCommand = $derived(`traceway login --url '${origin.replaceAll("'", "'\\''")}'`);
	const issueURL = $derived(
		`${origin}/issues/${hash}?projectId=${encodeURIComponent(projectsState.currentProjectId ?? '')}`
	);
	const prompt = $derived(
		`Use the traceway skill to investigate and fix this issue in the current repository: ${issueURL}\n\nTraceway project: ${projectsState.currentProjectId ?? ''}\nException hash: ${hash}\n\nFind the root cause using Traceway telemetry, make the fix, run the relevant tests, and explain the changes.`
	);
</script>

<AlertDialog.Root {open} onOpenChange={(value) => (open = value)}>
	<AlertDialog.Content
		class="w-[calc(100%-2rem)] max-w-2xl grid-cols-1"
		data-testid="local-fix-dialog"
	>
		<AlertDialog.Header>
			<AlertDialog.Title>Fix with Local LLM</AlertDialog.Title>
			<AlertDialog.Description>
				Connect your coding agent to Traceway, then work on this issue from your own editor or
				terminal.
			</AlertDialog.Description>
		</AlertDialog.Header>

		<Tabs.Root bind:value={harness} class="min-w-0">
			<Tabs.List
				class={`${underlineTabListClass} flex-wrap gap-x-5 gap-y-2`}
				aria-label="Coding agent"
			>
				{#each harnesses as item (item.id)}
					<Tabs.Trigger value={item.id} class={underlineTabTriggerClass}>{item.name}</Tabs.Trigger>
				{/each}
			</Tabs.List>
			{#each harnesses as item (item.id)}
				<Tabs.Content value={item.id} class="mt-4 space-y-5">
					<div class="space-y-2">
						<p class="text-sm font-medium">
							<span class="mr-2 text-muted-foreground">1</span>Install the Traceway skill
						</p>
						<p class="text-xs text-muted-foreground">
							Run in your repository’s terminal with Node.js installed, then start a new {item.name ===
							'VS Code'
								? 'GitHub Copilot agent chat in VS Code'
								: `${item.name} session`}.
						</p>
						<CopyableCodeBlock code={installCommand} language={bash} wrap />
					</div>
					<div class="space-y-2">
						<p class="text-sm font-medium">
							<span class="mr-2 text-muted-foreground">2</span>Connect to this instance
						</p>
						<p class="text-xs text-muted-foreground">
							Sign in with your Traceway account. Your credentials stay out of the prompt.
						</p>
						<CopyableCodeBlock code={loginCommand} language={bash} wrap />
						<a
							href="https://docs.tracewayapp.com/learn/cli#installation"
							target="_blank"
							rel="noopener noreferrer"
							class="inline-flex items-center gap-1 text-xs text-primary hover:underline"
						>
							Install the Traceway CLI first if needed <ExternalLink class="h-3 w-3" />
						</a>
					</div>
					<div class="space-y-2">
						<p class="text-sm font-medium">
							<span class="mr-2 text-muted-foreground">3</span>Ask {item.name === 'VS Code'
								? 'GitHub Copilot'
								: item.name} to fix this issue
						</p>
						<CopyablePrompt parts={[{ text: prompt, bold: false }]} />
					</div>
				</Tabs.Content>
			{/each}
		</Tabs.Root>

		<AlertDialog.Footer>
			<AlertDialog.Cancel><Terminal class="mr-2 h-4 w-4" />Done</AlertDialog.Cancel>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
