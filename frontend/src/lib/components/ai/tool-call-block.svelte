<script lang="ts">
	import { Wrench, ChevronRight } from 'lucide-svelte';
	import {
		tryParseJson,
		getMessageText,
		type ToolCall,
		type ChatMessage
	} from '$lib/utils/ai-conversation';

	type Props = {
		call: ToolCall;
		result?: ChatMessage;
	};

	let { call, result }: Props = $props();

	const formattedArguments = $derived.by(() => {
		const parsed = tryParseJson(call.arguments);
		return parsed ? JSON.stringify(parsed, null, 2) : call.arguments;
	});

	const resultText = $derived(result ? getMessageText(result.content) : '');

	let showArguments = $state(false);
	let showResult = $state(false);

	$effect(() => {
		if (resultText && resultText.length < 300) showResult = true;
	});
</script>

<div class="mt-1 rounded-md border bg-muted/40">
	<button
		type="button"
		class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm"
		onclick={() => (showArguments = !showArguments)}
	>
		<Wrench class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
		<span class="font-mono text-xs font-medium">{call.name}</span>
		<ChevronRight
			class="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform {showArguments
				? 'rotate-90'
				: ''}"
		/>
	</button>
	{#if showArguments && formattedArguments}
		<div class="border-t px-3 py-2">
			<p class="mb-1 text-[0.65rem] font-medium tracking-wide text-muted-foreground uppercase">
				Arguments
			</p>
			<pre class="max-h-64 overflow-auto font-mono text-xs break-words whitespace-pre-wrap">{formattedArguments}</pre>
		</div>
	{/if}
	{#if resultText}
		<div class="border-t px-3 py-2">
			<button
				type="button"
				class="flex w-full items-center gap-1 text-left"
				onclick={() => (showResult = !showResult)}
			>
				<p class="text-[0.65rem] font-medium tracking-wide text-muted-foreground uppercase">
					Result
				</p>
				<ChevronRight
					class="h-3 w-3 text-muted-foreground transition-transform {showResult ? 'rotate-90' : ''}"
				/>
			</button>
			{#if showResult}
				<pre class="mt-1 max-h-64 overflow-auto font-mono text-xs break-words whitespace-pre-wrap">{resultText}</pre>
			{/if}
		</div>
	{/if}
</div>
