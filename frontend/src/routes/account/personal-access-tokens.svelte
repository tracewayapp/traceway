<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import { Badge } from '$lib/components/ui/badge';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import StatusPill from '$lib/components/traceway/status-pill.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import { Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { formatDateTime } from '$lib/utils/formatters';
	import {
		personalAccessTokensState,
		type PersonalAccessToken
	} from '$lib/state/personal-access-tokens.svelte';
	import CreateTokenDialog from './create-token-dialog.svelte';

	let showCreate = $state(false);

	export function openCreate() {
		showCreate = true;
	}
	let tokenToRevoke = $state<PersonalAccessToken | null>(null);
	let revokeDialogOpen = $state(false);
	let revoking = $state(false);
	let revokeError = $state('');

	onMount(() => personalAccessTokensState.load());

	function formatDate(value: string | null): string {
		if (!value) return '';
		return formatDateTime(value, { format: 'date' });
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
			toast.success('Successfully revoked the Token');
			revokeDialogOpen = false;
			tokenToRevoke = null;
		} catch (e: unknown) {
			revokeError = e instanceof Error ? e.message : 'Failed to revoke token';
		} finally {
			revoking = false;
		}
	}
</script>

<InfoCallout>
	Authenticate the Traceway CLI or scripts without a browser. Treat tokens like passwords.
</InfoCallout>

{#if personalAccessTokensState.loading}
	<div class="flex justify-center py-12"><LoadingCircle size="lg" /></div>
{:else if personalAccessTokensState.error}
	<ErrorRetryBox
		message={personalAccessTokensState.error}
		onRetry={() => personalAccessTokensState.load()}
	/>
{:else if personalAccessTokensState.tokens.length === 0}
	<EmptyState
		message="No tokens yet. Create one to get started."
		actionLabel="Create your first Token"
		onAction={() => (showCreate = true)}
	/>
{:else}
	<TableContainer>
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
								<StatusPill tone="warning" label="Expired" />
							{:else}
								{formatDate(token.expiresAt)}
							{/if}
						</Table.Cell>
						<Table.Cell>
							<Button
								variant="ghost"
								size="icon"
								onclick={() => {
									revokeError = '';
									tokenToRevoke = token;
									revokeDialogOpen = true;
								}}
							>
								<Trash2 class="h-4 w-4 text-destructive" />
							</Button>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</TableContainer>
{/if}

<CreateTokenDialog bind:open={showCreate} />

<ConfirmDeleteDialog
	bind:open={revokeDialogOpen}
	entity="Token"
	verb="Revoke"
	description={`This permanently revokes ${tokenToRevoke?.name ? `"${tokenToRevoke.name}"` : 'this token'}. Any CLI or script using it will stop working immediately.`}
	error={revokeError}
	loading={revoking}
	onConfirm={confirmRevoke}
/>
