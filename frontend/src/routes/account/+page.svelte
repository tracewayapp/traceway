<script lang="ts">
	import PersonalAccessTokens from './personal-access-tokens.svelte';
	import ContactMethods from './contact-methods.svelte';
	import NotificationRules from './notification-rules.svelte';

	let methodsVersion = $state(0);
</script>

<div class="space-y-4">
	<div>
		<h1 class="text-3xl font-semibold tracking-tight">Account</h1>
	</div>
	<PersonalAccessTokens />
	<ContactMethods onMethodsChanged={() => (methodsVersion += 1)} />
	{#key methodsVersion}
		<NotificationRules />
	{/key}
</div>
