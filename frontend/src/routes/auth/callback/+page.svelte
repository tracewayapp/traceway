<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { projectsState } from '$lib/state/projects.svelte';
	import { consumeSsoReturnTo, gotoHref, safeLocalPath } from '$lib/utils/navigation';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';

	let error = $state('');

	onMount(async () => {
		const fragment = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : '';
		const params = new URLSearchParams(fragment);
		const token = params.get('token');
		const needsSetup = params.get('needsSetup') === 'true';

		history.replaceState(null, '', window.location.pathname);

		if (!token) {
			error = 'Missing authentication token.';
			setTimeout(() => gotoHref('/login?error=oauth_failed'), 1500);
			return;
		}

		authState.setToken(token);

		if (needsSetup) {
			// Leave the stashed returnTo in place: finish-setup consumes it.
			goto(resolve('/finish-setup'));
			return;
		}

		try {
			const response = await fetch('/api/me/login-bundle', {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (!response.ok) {
				throw new Error('Failed to load account');
			}
			const data = await response.json();
			authState.setOrganizations(data.organizations || []);
			projectsState.setProjects(data.projects || []);
			gotoHref(safeLocalPath(consumeSsoReturnTo()));
		} catch {
			authState.logout();
			error = 'Failed to load your account. Please try logging in again.';
			setTimeout(() => gotoHref('/login?error=oauth_failed'), 1500);
		}
	});
</script>

<div class="flex h-screen w-full items-center justify-center px-4">
	{#if error}
		<ErrorAlert {error} class="max-w-md" />
	{:else}
		<LoadingCircle size="xlg" />
	{/if}
</div>
