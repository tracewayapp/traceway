<script lang="ts">
	import ToolbarSelect from '$lib/components/traceway/toolbar-select.svelte';

	type Props = {
		value?: string;
		class?: string;
	};

	let { value = $bindable('all'), class: className = 'w-[120px]' }: Props = $props();

	const options = [
		{ value: 'all', label: 'All' },
		{ value: 'root', label: 'Root' },
		{ value: 'non_root', label: 'Non-root' }
	];
</script>

<ToolbarSelect bind:value {options} class={className} />
