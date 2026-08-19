<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { X } from '@lucide/svelte';

	interface Props {
		tags: string[];
		disabled?: boolean;
		placeholder?: string;
	}

	let {
		tags = $bindable(),
		disabled = false,
		placeholder = 'Add a tag and press Enter'
	}: Props = $props();

	let draft = $state('');

	function addDraft() {
		const tag = draft
			.trim()
			.toLowerCase()
			.replaceAll('"', '')
			.replaceAll('\\', '')
			.replaceAll(',', '');
		draft = '';
		if (!tag || tags.includes(tag)) return;
		tags = [...tags, tag];
	}

	function removeTag(tag: string) {
		tags = tags.filter((t) => t !== tag);
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			addDraft();
		} else if (event.key === 'Backspace' && draft === '' && tags.length > 0) {
			tags = tags.slice(0, -1);
		}
	}
</script>

<div
	class="flex min-h-9 w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-transparent px-2 py-1.5 shadow-xs focus-within:ring-1 focus-within:ring-ring"
>
	{#each tags as tag (tag)}
		<Badge variant="secondary" class="gap-1">
			{tag}
			{#if !disabled}
				<button
					type="button"
					class="cursor-pointer rounded-full opacity-60 hover:opacity-100"
					onclick={() => removeTag(tag)}
					aria-label={`Remove tag ${tag}`}
				>
					<X class="size-3" />
				</button>
			{/if}
		</Badge>
	{/each}
	{#if !disabled}
		<input
			type="text"
			bind:value={draft}
			onkeydown={onKeydown}
			onblur={addDraft}
			{placeholder}
			class="min-w-[140px] flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
		/>
	{:else if tags.length === 0}
		<span class="text-sm text-muted-foreground">No tags</span>
	{/if}
</div>
