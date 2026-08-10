<script lang="ts">
	import PersonalAccessTokens from './personal-access-tokens.svelte';
	import ContactMethods from './contact-methods.svelte';
	import NotificationRules from './notification-rules.svelte';
</script>

<div class="space-y-4">
	<div>
		<h1 class="text-3xl font-semibold tracking-tight">Account</h1>
	</div>
	<PersonalAccessTokens />
	<ContactMethods />
	<NotificationRules />
</div>
