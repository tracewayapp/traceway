<script lang="ts">
	import { resolve as resolveRoute } from '$app/paths';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { CircleAlert, ShieldCheck } from '@lucide/svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { themeState } from '$lib/state/theme.svelte';

	type Status =
		| 'loading'
		| 'needLogin'
		| 'confirm'
		| 'redirecting'
		| 'invalid'
		| 'unknownClient'
		| 'error';

	let status = $state<Status>('loading');
	let error = $state('');
	let clientName = $state('');

	const params = $derived({
		responseType: page.url.searchParams.get('response_type'),
		clientId: page.url.searchParams.get('client_id'),
		redirectUri: page.url.searchParams.get('redirect_uri'),
		state: page.url.searchParams.get('state') ?? '',
		codeChallenge: page.url.searchParams.get('code_challenge'),
		codeChallengeMethod: page.url.searchParams.get('code_challenge_method'),
		resource: page.url.searchParams.get('resource') ?? ''
	});

	const redirectHost = $derived.by(() => {
		try {
			return new URL(params.redirectUri ?? '').host || params.redirectUri;
		} catch {
			return params.redirectUri;
		}
	});

	function loginRedirect() {
		const target = page.url.pathname + page.url.search;
		goto(resolveRoute(`/login?returnTo=${encodeURIComponent(target)}`));
	}

	async function lookupClient() {
		status = 'loading';
		error = '';
		try {
			const res = await fetch(
				`/api/oauth/client?client_id=${encodeURIComponent(params.clientId ?? '')}`,
				{
					headers: { Authorization: `Bearer ${authState.token}` }
				}
			);
			if (res.status === 401) {
				authState.logout();
				loginRedirect();
				return;
			}
			if (res.status === 404) {
				status = 'unknownClient';
				return;
			}
			if (!res.ok) {
				status = 'error';
				error = 'We could not look up the requesting application.';
				return;
			}
			const data = await res.json();
			clientName = data.clientName || 'An application';
			status = 'confirm';
		} catch {
			status = 'error';
			error = 'We could not reach the server. Please try again.';
		}
	}

	async function resolve(approve: boolean) {
		status = 'loading';
		error = '';
		try {
			const body = approve
				? {
						client_id: params.clientId,
						redirect_uri: params.redirectUri,
						state: params.state,
						code_challenge: params.codeChallenge,
						code_challenge_method: params.codeChallengeMethod,
						resource: params.resource
					}
				: { client_id: params.clientId, redirect_uri: params.redirectUri, state: params.state };
			const res = await fetch(`/api/oauth/${approve ? 'approve' : 'deny'}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${authState.token}`
				},
				body: JSON.stringify(body)
			});
			if (res.status === 401) {
				authState.logout();
				loginRedirect();
				return;
			}
			if (!res.ok) {
				status = 'error';
				error =
					'The authorization request was rejected. Restart the connection from your application.';
				return;
			}
			const data = await res.json();
			status = 'redirecting';
			window.location.href = data.redirectTo;
		} catch {
			status = 'error';
			error = 'We could not reach the server. Please try again.';
		}
	}

	onMount(() => {
		if (!authState.isAuthenticated) {
			status = 'needLogin';
			return;
		}
		if (
			params.responseType !== 'code' ||
			!params.clientId ||
			!params.redirectUri ||
			!params.codeChallenge ||
			params.codeChallengeMethod !== 'S256'
		) {
			status = 'invalid';
			return;
		}
		lookupClient();
	});
</script>

<div class="flex min-h-screen w-full items-center justify-center px-4 py-8">
	<Card class="w-[420px]">
		<CardHeader>
			<CardTitle class="text-2xl">
				<div class="flex flex-row items-center justify-center gap-2">
					{#if themeState.isDark}
						<img src="/traceway-logo-white.svg" alt="Traceway Logo" class="h-8 w-auto" />
					{:else}
						<img src="/traceway-logo.png" alt="Traceway Logo" class="h-8 w-auto" />
					{/if}
				</div>
			</CardTitle>
			<CardDescription class="text-center">Authorize application</CardDescription>
		</CardHeader>
		<CardContent>
			{#if status === 'loading' || status === 'redirecting'}
				<div class="flex items-center justify-center py-8">
					<div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary"></div>
				</div>
			{:else if status === 'needLogin'}
				<div class="space-y-4">
					<p class="text-center text-muted-foreground">
						Log in to review an application's access request.
					</p>
					<Button class="w-full" onclick={loginRedirect}>Log in to continue</Button>
				</div>
			{:else if status === 'confirm'}
				<div class="space-y-4">
					<p class="text-center text-muted-foreground">
						<strong>{clientName}</strong> is requesting access to your Traceway account.
					</p>
					<p class="text-center text-sm text-muted-foreground">
						It will be able to read your projects' telemetry and act with your permissions. The
						application identified itself with this name; only approve if you just started
						connecting it, and it will send you back to <strong>{redirectHost}</strong>.
					</p>
					<div class="flex gap-2">
						<Button class="flex-1" onclick={() => resolve(true)}>
							<ShieldCheck class="mr-2 h-4 w-4" />
							Approve
						</Button>
						<Button variant="outline" class="flex-1" onclick={() => resolve(false)}>Deny</Button>
					</div>
				</div>
			{:else if status === 'invalid'}
				<Alert variant="destructive">
					<CircleAlert class="h-4 w-4" />
					<AlertTitle>Invalid authorization request</AlertTitle>
					<AlertDescription>
						The request is missing required parameters or uses an unsupported flow. Restart the
						connection from your application.
					</AlertDescription>
				</Alert>
			{:else if status === 'unknownClient'}
				<Alert variant="destructive">
					<CircleAlert class="h-4 w-4" />
					<AlertTitle>Unknown application</AlertTitle>
					<AlertDescription>
						We don't recognize the application making this request. Restart the connection from your
						application.
					</AlertDescription>
				</Alert>
			{:else}
				<ErrorAlert title="Something went wrong" error={error || 'Please try again.'} />
			{/if}
		</CardContent>
	</Card>
</div>
