<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Plus, Trash2, Info } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		personalAccessTokensState,
		type PersonalAccessToken
	} from '$lib/state/personal-access-tokens.svelte';
	import CreateTokenDialog from './create-token-dialog.svelte';

	let showCreate = $state(false);
	let tokenToRevoke = $state<PersonalAccessToken | null>(null);
	let revoking = $state(false);
	let revokeError = $state('');

	onMount(() => personalAccessTokensState.load());

	function formatDate(value: string | null): string {
		if (!value) return '';
		return new Date(value).toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	function isExpired(value: string | null): boolean {
		return !!value && new Date(value).getTime() < Date.now();
	}

	async function confirmRevoke() {
		if (!tokenToRevoke) return;
		revoking = true;
		revokeError = '';
		try {
			await personalAccessTokensState.revoke(tokenToRevoke.id);
			toast.success('Successfully revoked the Token', { position: 'top-center' });
			tokenToRevoke = null;
		} catch (e: unknown) {
			revokeError = e instanceof Error ? e.message : 'Failed to revoke token';
		} finally {
			revoking = false;
		}
	}
</script>

<div class="flex items-center justify-between">
	<h2 class="text-xl font-semibold tracking-tight">Personal Access Tokens</h2>
	<Button variant="success" onclick={() => (showCreate = true)}>
		<Plus class="mr-1 h-4 w-4" /> New Token
	</Button>
</div>

<Alert.Root
	class="border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-200"
>
	<Info class="text-blue-600 dark:text-blue-400" />
	<Alert.Description class="text-blue-800 dark:text-blue-300">
		Authenticate the Traceway CLI or scripts without a browser. Treat tokens like passwords.
	</Alert.Description>
</Alert.Root>

{#if personalAccessTokensState.loading}
	<div class="flex justify-center py-12"><LoadingCircle size="lg" /></div>
{:else if personalAccessTokensState.tokens.length === 0}
	<div
		class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
	>
		<p class="mb-4">No tokens yet. Create one to get started.</p>
		<Button variant="success" onclick={() => (showCreate = true)}>
			<Plus class="mr-1 h-4 w-4" />
			Create your first Token
		</Button>
	</div>
{:else}
	<div class="overflow-hidden rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row class="hover:bg-transparent">
					<Table.Head>Name</Table.Head>
					<Table.Head>Prefix</Table.Head>
					<Table.Head>Created</Table.Head>
					<Table.Head>Last used</Table.Head>
					<Table.Head>Expires</Table.Head>
					<Table.Head class="w-[80px]"></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each personalAccessTokensState.tokens as token (token.id)}
					<Table.Row>
						<Table.Cell class="font-medium">{token.name}</Table.Cell>
						<Table.Cell>
							<code class="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">{token.prefix}…</code>
						</Table.Cell>
						<Table.Cell>{formatDate(token.createdAt)}</Table.Cell>
						<Table.Cell>
							{#if token.lastUsedAt}
								{formatDate(token.lastUsedAt)}
							{:else}
								<Badge variant="outline">Never</Badge>
							{/if}
						</Table.Cell>
						<Table.Cell>
							{#if !token.expiresAt}
								<span class="text-muted-foreground">Never</span>
							{:else if isExpired(token.expiresAt)}
								<Badge variant="outline" class="border-amber-200 bg-amber-50 text-amber-700"
									>Expired</Badge
								>
							{:else}
								{formatDate(token.expiresAt)}
							{/if}
						</Table.Cell>
						<Table.Cell>
							<Button variant="ghost" size="icon" onclick={() => (tokenToRevoke = token)}>
								<Trash2 class="h-4 w-4 text-destructive" />
							</Button>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

<CreateTokenDialog bind:open={showCreate} />

<AlertDialog.Root
	open={tokenToRevoke !== null}
	onOpenChange={(open) => {
		if (!open) {
			tokenToRevoke = null;
			revokeError = '';
		}
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke Token</AlertDialog.Title>
			<AlertDialog.Description>
				This permanently revokes {tokenToRevoke?.name ? `"${tokenToRevoke.name}"` : 'this token'}.
				Any CLI or script using it will stop working immediately.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={revokeError} />
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={revoking}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmRevoke} disabled={revoking}>
				<Trash2 class="mr-2 h-4 w-4" />
				{revoking ? 'Revoking…' : 'Revoke Token'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
