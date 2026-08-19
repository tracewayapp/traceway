<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { onMount } from 'svelte';
	import { stashSsoReturnTo } from '$lib/utils/navigation';

	let { passwordLoginEnabled = $bindable(true), loaded = $bindable(false) } = $props();

	let providers = $state<string[]>([]);
	let providerLabels = $state<Record<string, string>>({});

	onMount(async () => {
		try {
			const response = await fetch('/api/auth/providers');
			if (response.ok) {
				const data = await response.json();
				providers = data.providers || [];
				providerLabels = data.providerLabels || {};
				passwordLoginEnabled = data.passwordLoginEnabled ?? true;
			}
		} catch {
			providers = [];
		} finally {
			loaded = true;
		}
	});

	function start(provider: string) {
		stashSsoReturnTo(new URLSearchParams(window.location.search).get('returnTo'));
		window.location.href = `/api/auth/start/${provider}`;
	}

	function getLabel(provider: string): string {
		if (provider === 'google') return 'Continue with Google';
		if (provider === 'github') return 'Continue with GitHub';
		const custom = providerLabels[provider];
		if (provider === 'oidc') return `Continue with ${custom ?? 'SSO'}`;
		return `Continue with ${custom ?? provider}`;
	}
</script>

{#if loaded && providers.length > 0}
	<div class="flex flex-col gap-2">
		{#each providers as provider, __index (__index)}
			<Button type="button" variant="outline" class="w-full" onclick={() => start(provider)}>
				{#if provider === 'google'}
					<svg class="size-4" viewBox="0 0 24 24" aria-hidden="true">
						<path
							fill="#4285F4"
							d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
						/>
						<path
							fill="#34A853"
							d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
						/>
						<path
							fill="#FBBC05"
							d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
						/>
						<path
							fill="#EA4335"
							d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
						/>
					</svg>
				{:else if provider === 'github'}
					<svg class="size-4" viewBox="0 0 24 24" aria-hidden="true" fill="currentColor">
						<path
							d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56v-2.16c-3.2.7-3.87-1.37-3.87-1.37-.52-1.32-1.27-1.67-1.27-1.67-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.69 1.24 3.34.95.1-.74.4-1.24.72-1.53-2.55-.29-5.24-1.27-5.24-5.66 0-1.25.45-2.27 1.18-3.07-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.15 1.17.91-.25 1.89-.38 2.86-.39.97 0 1.95.13 2.86.39 2.18-1.48 3.14-1.17 3.14-1.17.62 1.59.23 2.76.11 3.05.74.8 1.18 1.82 1.18 3.07 0 4.4-2.69 5.36-5.25 5.65.41.36.78 1.07.78 2.15v3.18c0 .31.21.67.8.56C20.21 21.39 23.5 17.08 23.5 12 23.5 5.65 18.35.5 12 .5z"
						/>
					</svg>
				{/if}
				{getLabel(provider)}
			</Button>
		{/each}
		{#if passwordLoginEnabled}
			<div class="my-2 flex items-center gap-3">
				<div class="flex-1 border-t"></div>
				<p class="text-xs text-muted-foreground">or</p>
				<div class="flex-1 border-t"></div>
			</div>
		{/if}
	</div>
{/if}
