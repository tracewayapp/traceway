<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Select from '$lib/components/ui/select';
	import { Plus, Check, ExternalLink } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { SECRET_SENTINEL } from '$lib/types/notifications';
	import type { Integration, ProviderInfo } from '$lib/types/agent';

	interface Props {
		open: boolean;
		organizationId: number;
		providers: ProviderInfo[];
		integration: Integration | null;
		onSaved: () => void;
	}

	let startingSetup = $state(false);

	async function startSetup(url: string) {
		startingSetup = true;
		error = '';
		try {
			const res = await api.post(url, { organizationId, integrationId: integration?.id ?? 0 });
			window.open(res.url, '_blank', 'noopener');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to start the setup';
		} finally {
			startingSetup = false;
		}
	}

	let { open = $bindable(), organizationId, providers, integration, onSaved }: Props = $props();

	let provider = $state('');
	let name = $state('');
	let values = $state<Record<string, string>>({});
	let loading = $state(false);
	let error = $state('');

	const isEditing = $derived(integration !== null);
	const selected = $derived(providers.find((p) => p.provider === provider) ?? null);
	const storedSecrets = $derived(integration?.hasSecrets ?? []);

	function reset() {
		error = '';
		if (integration) {
			provider = integration.provider;
			name = integration.name;
			const next: Record<string, string> = {};
			for (const [key, value] of Object.entries(integration.config ?? {})) {
				next[key] = value === SECRET_SENTINEL ? '' : value;
			}
			values = next;
		} else {
			provider = providers[0]?.provider ?? '';
			name = '';
			values = {};
		}
	}

	$effect(() => {
		if (open) reset();
	});

	function secretPlaceholder(key: string, fallback: string): string {
		return isEditing && storedSecrets.includes(key) ? 'Secret set, leave blank to keep' : fallback;
	}

	function buildConfig(): Record<string, string> {
		const config: Record<string, string> = {};
		for (const field of selected?.fields ?? []) {
			const typed = values[field.key] ?? '';
			if (field.kind === 'secret' && !typed && isEditing && storedSecrets.includes(field.key)) {
				config[field.key] = SECRET_SENTINEL;
			} else {
				config[field.key] = typed;
			}
		}
		return config;
	}

	async function submit() {
		loading = true;
		error = '';
		try {
			if (isEditing && integration) {
				await api.put(`/organizations/${organizationId}/integrations/${integration.id}`, {
					name,
					config: buildConfig()
				});
				toast.success('Successfully updated the Integration', { position: 'top-center' });
			} else {
				await api.post(`/organizations/${organizationId}/integrations`, {
					provider,
					name,
					config: buildConfig()
				});
				toast.success('Successfully created the Integration', { position: 'top-center' });
			}
			open = false;
			onSaved();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save the integration';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(value) => (open = value)}>
	<AlertDialog.Content class="max-h-[90vh] max-w-md overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Integration' : 'New Integration'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the connection to this provider'
					: 'Connect a provider the agent talks to: a code host, a chat tool'}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			class="space-y-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<ErrorAlert {error} />

			{#if !isEditing}
				<div class="space-y-2">
					<Label>Provider</Label>
					<Select.Root type="single" bind:value={provider}>
						<Select.Trigger class="w-full">{selected?.provider ?? 'Select provider'}</Select.Trigger
						>
						<Select.Content>
							{#each providers as info (info.provider)}
								<Select.Item value={info.provider}
									>{info.provider} ({info.kinds.join(', ')})</Select.Item
								>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			{/if}

			<div class="space-y-2">
				<Label for="integration-name">Name</Label>
				<Input id="integration-name" bind:value={name} placeholder="e.g. acme GitHub" required />
			</div>

			{#each selected?.fields ?? [] as field (field.key)}
				<div class="space-y-2">
					<Label for="integration-{field.key}">{field.label}</Label>
					{#if field.kind === 'select'}
						<Select.Root
							type="single"
							value={values[field.key] ?? ''}
							onValueChange={(v) => (values = { ...values, [field.key]: v ?? '' })}
						>
							<Select.Trigger class="w-full">{values[field.key] || 'Select'}</Select.Trigger>
							<Select.Content>
								{#each field.options ?? [] as option (option)}
									<Select.Item value={option}>{option}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{:else}
						<Input
							id="integration-{field.key}"
							type={field.kind === 'secret' ? 'password' : field.kind === 'url' ? 'url' : 'text'}
							value={values[field.key] ?? ''}
							oninput={(e) =>
								(values = { ...values, [field.key]: (e.currentTarget as HTMLInputElement).value })}
							placeholder={field.kind === 'secret' ? secretPlaceholder(field.key, '') : ''}
							required={field.required &&
								!(field.kind === 'secret' && isEditing && storedSecrets.includes(field.key))}
						/>
					{/if}
					{#if field.help}
						<p class="text-xs text-muted-foreground">{field.help}</p>
					{/if}
				</div>
			{/each}

			{#if selected?.setupFlow}
				<Button
					type="button"
					variant="outline"
					onclick={() => startSetup(selected.setupFlow!.url)}
					disabled={startingSetup}
					data-testid="setup-flow"
				>
					<ExternalLink class="mr-2 h-4 w-4" />
					{startingSetup ? 'Opening...' : selected.setupFlow.label}
				</Button>
				<p class="text-xs text-muted-foreground">
					Opens GitHub in a new tab. Save the integration first if you want the App to land on it;
					otherwise a new integration is created when GitHub returns.
				</p>
			{/if}
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={submit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Integration'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Integration'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
