<script lang="ts">
	import * as Alert from '$lib/components/ui/alert';
	import { TriangleAlert } from '@lucide/svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { sdkPlatform, SDK_PLATFORM_NAMES } from './sdk-config';

	const sdk = $derived(SDK_PLATFORM_NAMES[sdkPlatform(projectsState.currentProject?.framework)]);
</script>

<Alert.Root class="border-amber-500/40 bg-amber-500/5">
	<TriangleAlert />
	<Alert.Title>{sdk} SDK v2 required</Alert.Title>
	<Alert.Description>
		Remote client configuration (OpAMP), rules, and targeted captures work only with the Traceway
		{sdk} SDK v2. V1 clients continue sending data but do not apply these settings. Check the
		{sdk} SDK version in the session overview.
	</Alert.Description>
</Alert.Root>
