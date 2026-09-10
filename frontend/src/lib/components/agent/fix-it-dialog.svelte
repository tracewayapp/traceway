<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Select from '$lib/components/ui/select';
	import { Check, CircleAlert, Wrench, ExternalLink, RefreshCw } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { resolveHref } from '$lib/utils/links';
	import type { Attempt, Preflight } from '$lib/types/agent';

	interface Props {
		open: boolean;
		hash: string;
		onStarted?: (attempt: Attempt) => void;
	}

	let { open = $bindable(), hash, onStarted }: Props = $props();

	let preflight = $state<Preflight | null>(null);
	let loading = $state(false);
	let starting = $state(false);
	let error = $state('');
	let profileId = $state<string>('');
	let requestId = 0;

	async function loadPreflight() {
		const currentRequest = ++requestId;
		loading = true;
		error = '';
		try {
			const query = new SvelteURLSearchParams({ hash });
			if (profileId) query.set('profileId', profileId);
			const response: Preflight = await api.get(`/agent/preflight?${query}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			if (currentRequest !== requestId || !open) return;
			preflight = response;
			if (!profileId && response.defaultProfileId) profileId = String(response.defaultProfileId);
		} catch (e) {
			if (currentRequest !== requestId || !open) return;
			preflight = null;
			error = e instanceof Error ? e.message : 'Failed to check the setup';
		} finally {
			if (currentRequest === requestId) loading = false;
		}
	}

	$effect(() => {
		if (!open) return;
		untrack(() => {
			profileId = '';
			preflight = null;
			loadPreflight();
		});
		return () => {
			requestId += 1;
		};
	});

	async function start() {
		starting = true;
		error = '';
		try {
			const body: { hash: string; profileId?: number } = { hash };
			if (profileId) body.profileId = Number(profileId);
			const res = await api.post('/agent/attempts', body, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			if (res.existing) {
				toast.info('An attempt is already running for this issue', { position: 'top-center' });
			} else {
				toast.success('Successfully started the Attempt', { position: 'top-center' });
			}
			open = false;
			onStarted?.(res.attempt);
			goto(resolve(`/agent/${res.attempt.id}`));
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start the attempt';
		} finally {
			starting = false;
		}
	}

	const canStart = $derived(preflight?.canStart ?? false);
	const selectedProfile = $derived(
		preflight?.profiles.find((p) => String(p.id) === profileId) ?? null
	);
</script>

<AlertDialog.Root {open} onOpenChange={(value) => (open = value)}>
	<AlertDialog.Content class="max-w-lg">
		<AlertDialog.Header>
			<AlertDialog.Title>Fix with Traceway AI Agent</AlertDialog.Title>
			<AlertDialog.Description>
				Connect your GitHub repository and configure an available agent to investigate this issue
				and open a draft pull request.
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if loading}
			<div class="flex justify-center py-6"><LoadingCircle size="lg" /></div>
		{:else if preflight}
			<div class="space-y-3">
				<ErrorAlert {error} />
				<ul class="space-y-2" data-testid="preflight-checks">
					{#each preflight.checks as check (check.key)}
						<li class="flex items-start gap-3 rounded-md border p-3">
							{#if check.ok}
								<Check class="mt-0.5 h-4 w-4 shrink-0 text-green-600 dark:text-green-400" />
							{:else}
								<CircleAlert class="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
							{/if}
							<div class="min-w-0 flex-1 space-y-0.5">
								<p class="text-sm font-medium">{check.label}</p>
								{#if check.hint}
									<p class="text-xs text-muted-foreground">{check.hint}</p>
								{/if}
							</div>
							{#if !check.ok && check.href}
								<a
									{...{
										href: check.href.startsWith('http') ? check.href : resolveHref(check.href)
									}}
									target={check.href.startsWith('http') ? '_blank' : undefined}
									rel="noopener noreferrer"
									class="flex items-center gap-1 text-xs text-blue-600 hover:underline dark:text-blue-400"
								>
									{check.key === 'repository' ? 'Connect GitHub' : 'Set up'}
									<ExternalLink class="h-3 w-3" />
								</a>
							{/if}
						</li>
					{/each}
				</ul>

				{#if preflight.profiles.length > 0}
					<div class="space-y-2">
						<Label>Agent profile</Label>
						<Select.Root
							type="single"
							value={profileId}
							onValueChange={(value) => {
								profileId = value;
								loadPreflight();
							}}
						>
							<Select.Trigger class="w-full">
								{selectedProfile?.name ?? 'Select profile'}
							</Select.Trigger>
							<Select.Content>
								{#each preflight.profiles as profile (profile.id)}
									<Select.Item value={String(profile.id)}>
										{profile.name}{profile.isDefault ? ' (default)' : ''}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
				{/if}

				{#if preflight.activeAttempt}
					<p class="text-sm text-muted-foreground">
						Attempt {preflight.activeAttempt.number} is still {preflight.activeAttempt.status.replaceAll(
							'_',
							' '
						)}.
						<a
							{...{ href: resolveHref(`/agent/${preflight.activeAttempt.id}`) }}
							class="text-blue-600 hover:underline dark:text-blue-400">Open it</a
						>
					</p>
				{/if}
			</div>
		{:else}
			<ErrorAlert {error} />
		{/if}

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={starting}>Cancel</AlertDialog.Cancel>
			<Button variant="outline" onclick={loadPreflight} disabled={loading || starting}>
				<RefreshCw class="mr-2 h-4 w-4" />Check again
			</Button>
			{#if canStart}
				<Button variant="success" onclick={start} disabled={starting || loading}>
					<Wrench class="mr-2 h-4 w-4" />
					{#if starting}
						Starting...
					{:else}
						Start attempt {preflight?.nextNumber ?? ''}
					{/if}
				</Button>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
