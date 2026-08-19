<script lang="ts">
	import { page } from '$app/state';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import { setTabParam } from '$lib/utils/url-params';
	import PersonalAccessTokens from './personal-access-tokens.svelte';
	import ContactMethods from './contact-methods.svelte';
	import NotificationRules from './notification-rules.svelte';

	const TABS = [
		{ value: 'tokens', label: 'Personal Access Tokens' },
		{ value: 'contact-methods', label: 'Contact Methods' },
		{ value: 'notification-rules', label: 'Notification Rules' }
	];

	const activeTab = $derived.by(() => {
		const tab = page.url.searchParams.get('tab') || 'tokens';
		return TABS.some((t) => t.value === tab) ? tab : 'tokens';
	});

	let methodsVersion = $state(0);
	let tokensSection = $state<PersonalAccessTokens>();
	let methodsSection = $state<ContactMethods>();

	const newAction = $derived.by(() => {
		if (activeTab === 'tokens') {
			return { label: 'New Token', onclick: () => tokensSection?.openCreate() };
		}
		if (activeTab === 'contact-methods') {
			return { label: 'New Contact Method', onclick: () => methodsSection?.openCreate() };
		}
		return undefined;
	});
</script>

<div class="space-y-4">
	<PageHeader title="Account" />

	<PageTabs
		tabs={TABS}
		{activeTab}
		onTabChange={(tab) => setTabParam(tab)}
		actionLabel={newAction?.label}
		onAction={newAction?.onclick}
	/>

	{#if activeTab === 'tokens'}
		<PersonalAccessTokens bind:this={tokensSection} />
	{:else if activeTab === 'contact-methods'}
		<ContactMethods bind:this={methodsSection} onMethodsChanged={() => (methodsVersion += 1)} />
	{:else}
		{#key methodsVersion}
			<NotificationRules />
		{/key}
	{/if}
</div>
