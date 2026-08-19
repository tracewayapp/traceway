<script lang="ts">
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { addStickyParamsToHref } from '$lib/utils/navigation';

	type Props = {
		active: 'traces' | 'conversations' | 'users';
	};

	let { active }: Props = $props();

	const tabs = [
		{ value: 'traces', label: 'Traces', href: resolve('/ai-traces') },
		{ value: 'conversations', label: 'Conversations', href: resolve('/ai-traces/conversations') },
		{ value: 'users', label: 'Users', href: resolve('/ai-traces/users') }
	];

	function onTabChange(value: string) {
		const tab = tabs.find((t) => t.value === value);
		if (!tab || tab.value === active) return;
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(addStickyParamsToHref(tab.href, 'preset', 'from', 'to'));
	}
</script>

<PageTabs {tabs} activeTab={active} {onTabChange} />
