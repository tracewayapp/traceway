<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Plus, Trash2 } from '@lucide/svelte';
	import { cn } from '$lib/utils';
	import { REPLAY_BLOCK_CLASS } from '$lib/utils/self-monitoring';

	interface Props {
		rows: { key: string; value: string }[];
		keyPlaceholder?: string;
		valuePlaceholder?: string;
		addLabel?: string;
		disabled?: boolean;
		secretValues?: boolean;
	}

	let {
		rows = $bindable(),
		keyPlaceholder = 'Name',
		valuePlaceholder = 'Value',
		addLabel = 'Add',
		disabled = false,
		secretValues = false
	}: Props = $props();

	function addRow() {
		rows = [...rows, { key: '', value: '' }];
	}

	function removeRow(index: number) {
		rows = rows.filter((_, i) => i !== index);
	}
</script>

<div class="space-y-2">
	{#each rows as row, index (index)}
		<div class="flex gap-2">
			<Input bind:value={row.key} placeholder={keyPlaceholder} class="flex-1" {disabled} />
			<Input
				bind:value={row.value}
				placeholder={valuePlaceholder}
				class={cn('flex-1', secretValues && REPLAY_BLOCK_CLASS)}
				{disabled}
			/>
			<Button variant="ghost" size="icon" type="button" onclick={() => removeRow(index)} {disabled}>
				<Trash2 class="h-4 w-4" />
			</Button>
		</div>
	{/each}
	<Button variant="outline" size="sm" type="button" onclick={addRow} {disabled}>
		<Plus class="mr-1 h-3 w-3" />
		{addLabel}
	</Button>
</div>
