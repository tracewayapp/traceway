<script lang="ts">
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';

	type Props = {
		value?: string;
		onChange?: (value: string) => void;
		class?: string;
	};

	let { value = $bindable('all'), onChange, class: className = 'w-[130px]' }: Props = $props();

	const options = [
		{ value: 'all', label: 'All' },
		{ value: 'flagged', label: 'Flagged only' }
	];
</script>

<ToolbarSelect bind:value {options} class={className} onChange={(v) => onChange?.(v || 'all')} />
