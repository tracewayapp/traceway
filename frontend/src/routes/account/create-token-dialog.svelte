<script lang="ts">
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Plus, TriangleAlert } from '@lucide/svelte';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { toast } from 'svelte-sonner';
	import CopyableInline from '$lib/components/setup/copyable-inline.svelte';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import {
		personalAccessTokensState,
		type TokenScope
	} from '$lib/state/personal-access-tokens.svelte';

	let { open = $bindable() }: { open: boolean } = $props();

	let name = $state('');
	let expiry = $state('30');
	let scope = $state<TokenScope>('traceway:read');
	let loading = $state(false);
	let error = $state('');
	let createdToken = $state<string | null>(null);

	const scopeOptions: { value: TokenScope; label: string }[] = [
		{ value: 'traceway:read', label: 'Read only' },
		{ value: 'traceway:write', label: 'Read and write' }
	];

	const expiryOptions = $derived([
		{ value: '7', label: '7 days' },
		{ value: '30', label: '30 days' },
		{ value: '60', label: '60 days' },
		{ value: '90', label: '90 days' },
		...(scope === 'traceway:write' ? [] : [{ value: '0', label: 'No expiration' }])
	]);

	$effect(() => {
		if (scope === 'traceway:write' && expiry === '0') {
			expiry = '30';
		}
	});

	async function handleCreate() {
		loading = true;
		error = '';
		try {
			const days = parseInt(expiry, 10);
			const res = await personalAccessTokensState.create(
				name.trim(),
				days > 0 ? days : null,
				scope
			);
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
			scope = 'traceway:read';
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
				<Alert.Root
					class="border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
				>
					<TriangleAlert class="text-amber-600 dark:text-amber-400" />
					<Alert.Description class="text-amber-800 dark:text-amber-300">
						Store it somewhere safe. This is the only time it will be shown.
					</Alert.Description>
				</Alert.Root>
				<CopyableInline value={createdToken} />
			</div>
			<AlertDialog.Footer>
				<Button onclick={() => handleOpenChange(false)}>Done</Button>
			</AlertDialog.Footer>
		{:else}
			<AlertDialog.Header>
				<AlertDialog.Title>New Token</AlertDialog.Title>
				<AlertDialog.Description>
					Use this token to authenticate the Traceway CLI, an MCP client or scripts.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					handleCreate();
				}}
				class="space-y-4"
			>
				<ErrorAlert {error} />
				<div class="space-y-2">
					<Label for="token-name">Name</Label>
					<Input id="token-name" bind:value={name} placeholder="e.g. laptop CLI" required />
				</div>
				<div class="space-y-2">
					<Label for="token-scope">Access</Label>
					<Select.Root type="single" bind:value={scope}>
						<Select.Trigger id="token-scope" class="w-full">
							{scopeOptions.find((o) => o.value === scope)?.label}
						</Select.Trigger>
						<Select.Content>
							{#each scopeOptions as option (option.value)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				{#if scope === 'traceway:write'}
					<WarningCallout title="Write tokens change shared data">
						An agent holding this token can archive issues, resolve pages, snooze alerts and edit
						post-mortems as you. It only works in organizations where an admin turned on agent
						writes, and it must expire within 90 days.
					</WarningCallout>
				{/if}
				<div class="space-y-2">
					<Label for="token-expiry">Expiration</Label>
					<Select.Root type="single" bind:value={expiry}>
						<Select.Trigger class="w-full">
							{expiryOptions.find((o) => o.value === expiry)?.label || 'Select'}
						</Select.Trigger>
						<Select.Content>
							{#each expiryOptions as option, __index (__index)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
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
