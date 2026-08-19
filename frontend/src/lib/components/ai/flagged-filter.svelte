<script lang="ts">
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';

	type Props = {
		value?: string;
		onChange?: (value: string) => void;
	};

	let { value = $bindable('all'), onChange }: Props = $props();

	const options = [
		{ value: 'all', label: 'All' },
		{ value: 'flagged', label: 'Flagged only' }
	];
</script>

<ToolbarSelect bind:value {options} class="w-[130px]" onChange={(v) => onChange?.(v || 'all')} />
