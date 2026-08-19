<script lang="ts">
	import ToolCallBlock from './tool-call-block.svelte';
	import {
		getMessageText,
		getMessageImages,
		getRoleLabel,
		pairToolResults,
		type ChatMessage
	} from '$lib/utils/ai-conversation';

	type Props = {
		messages: ChatMessage[];
	};

	let { messages }: Props = $props();

	const toolResults = $derived(pairToolResults(messages));

	// Tool-result messages that render inside their call's ToolCallBlock are
	// hidden from the top-level list; unpaired ones still render standalone.
	const visibleMessages = $derived(
		messages.filter(
			(msg) =>
				!(
					(msg.role === 'tool' || msg.role === 'function') &&
					msg.toolCallId &&
					toolResults.get(msg.toolCallId) === msg &&
					messages.some((m) => m.toolCalls?.some((c) => c.id === msg.toolCallId))
				)
		)
	);
</script>

<div class="space-y-3">
	{#each visibleMessages as msg, __index (__index)}
		<div class="flex gap-3">
			<div
				class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-medium
				{msg.role === 'user'
					? 'bg-primary text-primary-foreground'
					: msg.role === 'assistant'
						? 'border bg-muted text-muted-foreground'
						: msg.role === 'system'
							? 'bg-amber-500/15 text-amber-700 dark:text-amber-400'
							: 'bg-muted text-muted-foreground'}"
			>
				{getRoleLabel(msg.role).charAt(0)}
			</div>
			<div class="min-w-0 flex-1">
				<p class="mb-1 text-xs font-medium text-muted-foreground">
					{getRoleLabel(msg.role)}
				</p>
				{#each getMessageImages(msg.content) as imageUrl, __index (__index)}
					{#if imageUrl && !imageUrl.includes('REDACTED')}
						<div class="mb-2 max-w-sm">
							<img
								src={imageUrl}
								alt="Message attachment"
								class="max-h-64 rounded-md border object-contain"
							/>
						</div>
					{/if}
				{/each}
				{#if getMessageText(msg.content)}
					<div
						class="rounded-md px-3 py-2 text-sm break-words whitespace-pre-wrap
						{msg.role === 'user'
							? 'bg-primary/10'
							: msg.role === 'system'
								? 'bg-amber-500/10 font-mono text-xs'
								: 'bg-muted'}"
					>
						{getMessageText(msg.content)}
					</div>
				{/if}
				{#each msg.toolCalls ?? [] as call, __index (__index)}
					<ToolCallBlock {call} result={call.id ? toolResults.get(call.id) : undefined} />
				{/each}
			</div>
		</div>
	{/each}
</div>
