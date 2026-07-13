<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Plus, TriangleAlert } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import CopyableInline from '$lib/components/setup/copyable-inline.svelte';
	import { personalAccessTokensState } from '$lib/state/personal-access-tokens.svelte';

	let { open = $bindable() }: { open: boolean } = $props();

	let name = $state('');
	let expiry = $state('30');
	let loading = $state(false);
	let error = $state('');
	let createdToken = $state<string | null>(null);

	const expiryOptions = [
		{ value: '30', label: '30 days' },
		{ value: '60', label: '60 days' },
		{ value: '90', label: '90 days' },
		{ value: '0', label: 'No expiration' }
	];

	async function handleCreate() {
		loading = true;
		error = '';
		try {
			const days = parseInt(expiry, 10);
			const res = await personalAccessTokensState.create(name.trim(), days > 0 ? days : null);
			createdToken = res.token;
			toast.success('Successfully created the Token', { position: 'top-center' });
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to create token';
		} finally {
			loading = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		if (!isOpen) {
			name = '';
			expiry = '30';
			error = '';
			createdToken = null;
		}
		open = isOpen;
	}
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
	<AlertDialog.Content class="max-w-md">
		{#if createdToken}
			<AlertDialog.Header>
				<AlertDialog.Title>Token created</AlertDialog.Title>
				<AlertDialog.Description>
					Copy your token now. For security, you won't be able to see it again.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<div class="space-y-3">
				<CopyableInline value={createdToken} />
				<p class="flex items-center gap-2 text-sm text-amber-600">
					<TriangleAlert class="h-4 w-4 shrink-0" />
					Store it somewhere safe — this is the only time it will be shown.
				</p>
			</div>
			<AlertDialog.Footer>
				<Button onclick={() => handleOpenChange(false)}>Done</Button>
			</AlertDialog.Footer>
		{:else}
			<AlertDialog.Header>
				<AlertDialog.Title>New Token</AlertDialog.Title>
				<AlertDialog.Description>
					Use this token to authenticate the Traceway CLI or scripts.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<form onsubmit={(e) => { e.preventDefault(); handleCreate(); }} class="space-y-4">
				<div class="space-y-2">
					<Label for="token-name">Name</Label>
					<Input id="token-name" bind:value={name} placeholder="e.g. laptop CLI" required />
				</div>
				<div class="space-y-2">
					<Label for="token-expiry">Expiration</Label>
					<Select.Root type="single" bind:value={expiry}>
						<Select.Trigger class="w-full">
							{expiryOptions.find((o) => o.value === expiry)?.label || 'Select'}
						</Select.Trigger>
						<Select.Content>
							{#each expiryOptions as option}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
			</form>
			<AlertDialog.Footer>
				<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
				<Button variant="success" onclick={handleCreate} disabled={loading}>
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating…' : 'New Token'}
				</Button>
			</AlertDialog.Footer>
		{/if}
	</AlertDialog.Content>
</AlertDialog.Root>
