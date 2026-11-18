<script lang="ts">
	import * as Alert from '$lib/components/ui/alert';
	import { TriangleAlert } from '@lucide/svelte';
</script>

<Alert.Root class="border-amber-500/40 bg-amber-500/5">
	<TriangleAlert />
	<Alert.Title>JavaScript SDK v2 required</Alert.Title>
	<Alert.Description>
		Remote client configuration (OpAMP), rules, and targeted captures work only with the Traceway
		JavaScript SDK v2. V1 clients continue sending data but do not apply these settings. Check the
		JavaScript SDK version in the session overview.
	</Alert.Description>
</Alert.Root>
