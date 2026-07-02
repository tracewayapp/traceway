<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
	import { CircleAlert, Check, ShieldCheck } from '@lucide/svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { themeState } from '$lib/state/theme.svelte';

	type Status =
		| 'loading'
		| 'needLogin'
		| 'needCode'
		| 'confirm'
		| 'approving'
		| 'approved'
		| 'denied'
		| 'notFound'
		| 'expired'
		| 'alreadyHandled'
		| 'error';

	let status = $state<Status>('loading');
	let error = $state('');
	let info = $state<{ userCode: string; clientName: string } | null>(null);
	let manualCode = $state('');

	const userCode = $derived(page.url.searchParams.get('user_code'));

	// Codes are generated uppercase in the XXXX-XXXX shape; users may paste them
	// lowercase, with spaces, or without the hyphen. Normalize to match the
	// server's exact lookup.
	function normalizeUserCode(raw: string): string {
		const cleaned = raw.toUpperCase().replace(/[^0-9A-Z]/g, '');
		return cleaned.length === 8 ? `${cleaned.slice(0, 4)}-${cleaned.slice(4)}` : cleaned;
	}

	function loginRedirect() {
		const target = `/device${userCode ? `?user_code=${encodeURIComponent(userCode)}` : ''}`;
		goto(`/login?returnTo=${encodeURIComponent(target)}`);
	}

	async function lookup(code: string) {
		status = 'loading';
		error = '';
		try {
			const res = await fetch(`/api/device?user_code=${encodeURIComponent(code)}`, {
				headers: { Authorization: `Bearer ${authState.token}` }
			});
			if (res.status === 401) {
				loginRedirect();
				return;
			}
			if (res.status === 404) {
				status = 'notFound';
				return;
			}
			if (res.status === 410) {
				status = 'expired';
				return;
			}
			if (!res.ok) {
				status = 'error';
				error = 'We could not look up that code.';
				return;
			}
			const data = await res.json();
			if (data.status === 'approved' || data.status === 'denied') {
				status = 'alreadyHandled';
				return;
			}
			if (data.status === 'expired') {
				status = 'expired';
				return;
			}
			info = { userCode: data.user_code, clientName: data.clientName || 'a device' };
			status = 'confirm';
		} catch {
			status = 'error';
			error = 'We could not reach the server. Please try again.';
		}
	}

	async function resolve(approve: boolean) {
		if (!info) return;
		status = 'approving';
		error = '';
		try {
			const res = await fetch(`/api/device/${approve ? 'approve' : 'deny'}`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${authState.token}`
				},
				body: JSON.stringify({ user_code: info.userCode })
			});
			if (!res.ok) {
				if (res.status === 401) {
					loginRedirect();
					return;
				}
				if (res.status === 404) {
					status = 'notFound';
					return;
				}
				if (res.status === 410) {
					status = 'expired';
					return;
				}
				status = 'error';
				error = 'We could not complete the request.';
				return;
			}
			status = approve ? 'approved' : 'denied';
		} catch {
			status = 'error';
			error = 'We could not reach the server. Please try again.';
		}
	}

	function submitManual(e: SubmitEvent) {
		e.preventDefault();
		const code = normalizeUserCode(manualCode);
		if (code) lookup(code);
	}

	onMount(() => {
		if (!authState.isAuthenticated) {
			status = 'needLogin';
			return;
		}
		if (userCode) {
			lookup(normalizeUserCode(userCode));
		} else {
			status = 'needCode';
		}
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
			<CardDescription class="text-center">Device login</CardDescription>
		</CardHeader>
		<CardContent>
			{#if status === 'loading' || status === 'approving'}
				<div class="flex items-center justify-center py-8">
					<div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary"></div>
				</div>
			{:else if status === 'needLogin'}
				<div class="space-y-4">
					<p class="text-center text-muted-foreground">
						Log in to confirm a device login on your account.
					</p>
					<Button class="w-full" onclick={loginRedirect}>Log in to continue</Button>
				</div>
			{:else if status === 'needCode'}
				<form onsubmit={submitManual} class="space-y-4">
					<div class="space-y-2">
						<Label for="code">Enter the code shown in your terminal</Label>
						<Input id="code" bind:value={manualCode} placeholder="XXXX-XXXX" autocomplete="off" />
					</div>
					<Button type="submit" class="w-full" disabled={!manualCode.trim()}>Continue</Button>
				</form>
			{:else if status === 'confirm' && info}
				<div class="space-y-4">
					<p class="text-center text-muted-foreground">
						<strong>{info.clientName}</strong> is requesting access to your Traceway account.
					</p>
					<div class="font-mono text-4xl font-semibold tracking-[0.3em] text-center py-4">
						{info.userCode}
					</div>
					<p class="text-center text-sm text-muted-foreground">
						Only approve if this matches the code shown in your terminal.
					</p>
					<div class="flex gap-2">
						<Button class="flex-1" onclick={() => resolve(true)}>
							<ShieldCheck class="mr-2 h-4 w-4" />
							Approve
						</Button>
						<Button variant="outline" class="flex-1" onclick={() => resolve(false)}>Deny</Button>
					</div>
				</div>
			{:else if status === 'approved'}
				<div class="flex flex-col items-center justify-center gap-4 py-8">
					<div class="rounded-full bg-green-100 p-3">
						<Check class="h-8 w-8 text-green-600" />
					</div>
					<p class="text-center text-lg font-medium">Device approved</p>
					<p class="text-center text-muted-foreground">You can return to your terminal.</p>
				</div>
			{:else if status === 'denied'}
				<Alert>
					<CircleAlert class="h-4 w-4" />
					<AlertTitle>Request denied</AlertTitle>
					<AlertDescription>The device login was denied. You can close this page.</AlertDescription>
				</Alert>
			{:else if status === 'notFound'}
				<Alert variant="destructive" class="border-red-200 bg-red-50">
					<CircleAlert class="h-4 w-4 text-red-700" />
					<AlertTitle class="text-red-800">Code not found</AlertTitle>
					<AlertDescription class="text-red-700">
						We couldn't find that code. Start a new login from your terminal.
					</AlertDescription>
				</Alert>
			{:else if status === 'expired'}
				<Alert variant="destructive" class="border-red-200 bg-red-50">
					<CircleAlert class="h-4 w-4 text-red-700" />
					<AlertTitle class="text-red-800">Code expired</AlertTitle>
					<AlertDescription class="text-red-700">
						This login request has expired. Start a new login from your terminal.
					</AlertDescription>
				</Alert>
			{:else if status === 'alreadyHandled'}
				<Alert>
					<CircleAlert class="h-4 w-4" />
					<AlertTitle>Already handled</AlertTitle>
					<AlertDescription>This login request was already approved or denied.</AlertDescription>
				</Alert>
			{:else}
				<Alert variant="destructive" class="border-red-200 bg-red-50">
					<CircleAlert class="h-4 w-4 text-red-700" />
					<AlertTitle class="text-red-800">Something went wrong</AlertTitle>
					<AlertDescription class="text-red-700">{error || 'Please try again.'}</AlertDescription>
				</Alert>
			{/if}
		</CardContent>
	</Card>
</div>
