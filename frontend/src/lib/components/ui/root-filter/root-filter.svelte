<script lang="ts">
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';

	type Props = {
		value?: string;
	};

	let { value = $bindable('all') }: Props = $props();

	const options = [
		{ value: 'all', label: 'All' },
		{ value: 'root', label: 'Root' },
		{ value: 'non_root', label: 'Non-root' }
	];
</script>

<ToolbarSelect bind:value {options} class="w-[120px]" />
