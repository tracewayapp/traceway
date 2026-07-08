<script lang="ts">
	import { onMount } from 'svelte';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import TableEmptyState from '$lib/components/ui/table-empty-state/table-empty-state.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Plus, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import {
		personalAccessTokensState,
		type PersonalAccessToken
	} from '$lib/state/personal-access-tokens.svelte';
	import CreateTokenDialog from './create-token-dialog.svelte';

	let showCreate = $state(false);
	let tokenToRevoke = $state<PersonalAccessToken | null>(null);
	let revoking = $state(false);

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
		try {
			await personalAccessTokensState.revoke(tokenToRevoke.id);
			toast.success('Successfully revoked the token', { position: 'top-center' });
			tokenToRevoke = null;
		} catch (e: unknown) {
			toast.error(e instanceof Error ? e.message : 'Failed to revoke token');
		} finally {
			revoking = false;
		}
	}
</script>

<Card class="pb-0">
	<CardHeader class="flex flex-row items-start justify-between gap-4 space-y-0 pb-4">
		<div class="min-w-0 space-y-1.5">
			<CardTitle>Personal Access Tokens</CardTitle>
			<CardDescription>
				Authenticate the Traceway CLI or scripts without a browser. Treat tokens like passwords.
			</CardDescription>
		</div>
		<Button variant="outline" class="shrink-0" onclick={() => (showCreate = true)}>
			<Plus class="mr-2 h-4 w-4" />
			Create token
		</Button>
	</CardHeader>
	<CardContent class="p-0">
		{#if personalAccessTokensState.loading}
			<div class="flex items-center justify-center py-12">
				<LoadingCircle size="lg" />
			</div>
		{:else}
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
					{#if personalAccessTokensState.tokens.length === 0}
						<TableEmptyState colspan={6} message="No tokens yet." />
					{:else}
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
										<Badge variant="outline" class="border-amber-200 bg-amber-50 text-amber-700">Expired</Badge>
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
					{/if}
				</Table.Body>
			</Table.Root>
		{/if}
	</CardContent>
</Card>

<CreateTokenDialog bind:open={showCreate} />

<AlertDialog.Root open={tokenToRevoke !== null} onOpenChange={(open) => { if (!open) tokenToRevoke = null; }}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke token</AlertDialog.Title>
			<AlertDialog.Description>
				This permanently revokes {tokenToRevoke?.name ? `"${tokenToRevoke.name}"` : 'this token'}. Any
				CLI or script using it will stop working immediately.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={revoking}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmRevoke} disabled={revoking}>
				{revoking ? 'Revoking…' : 'Revoke token'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
