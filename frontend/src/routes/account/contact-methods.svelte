<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import { Switch } from '$lib/components/ui/switch';
	import { Badge } from '$lib/components/ui/badge';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import {
		Trash2,
		Send,
		Mail,
		MessageSquare,
		Bell,
		Smartphone,
		ShieldCheck,
		Pencil
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { type ContactMethod } from '$lib/state/oncall.svelte';
	import ContactMethodDialog from './contact-method-dialog.svelte';
	import VerifyCodeDialog from './verify-code-dialog.svelte';

	interface Props {
		onMethodsChanged?: () => void;
	}

	let { onMethodsChanged }: Props = $props();

	let methods = $state<ContactMethod[]>([]);
	let smsEnabled = $state(false);
	let loading = $state(true);
	let loadError = $state('');
	let showCreate = $state(false);

	export function openCreate() {
		showCreate = true;
	}
	let showEdit = $state(false);
	let methodToEdit = $state<ContactMethod | null>(null);
	let methodToDelete = $state<ContactMethod | null>(null);
	let deleteDialogOpen = $state(false);
	let deleting = $state(false);
	let deleteError = $state('');
	let testingId = $state<number | null>(null);
	let showVerify = $state(false);
	let verifyMethod = $state<ContactMethod | null>(null);
	let verifyCodeJustSent = $state(false);

	const methodTypeLabels: Record<string, string> = {
		email: 'Email',
		slack: 'Slack',
		pushover: 'Pushover',
		telegram: 'Telegram',
		sms: 'SMS'
	};

	const methodTypeIcons: Record<string, typeof Mail> = {
		email: Mail,
		slack: MessageSquare,
		pushover: Bell,
		telegram: Send,
		sms: Smartphone
	};

	function isUnverifiedSms(method: ContactMethod): boolean {
		return method.methodType === 'sms' && !method.verified;
	}

	// An sms method can outlive its transport when Twilio credentials are
	// removed. It stays listed and deletable, but nothing can be sent through
	// it, so verifying and testing are closed off.
	function isUnsendableSms(method: ContactMethod): boolean {
		return method.methodType === 'sms' && !smsEnabled;
	}

	function openVerify(method: ContactMethod, codeJustSent = false) {
		verifyMethod = method;
		verifyCodeJustSent = codeJustSent;
		showVerify = true;
	}

	onMount(() => {
		loadMethods();
	});

	async function loadMethods() {
		loading = true;
		loadError = '';
		try {
			const res = await api.get('/contact-methods');
			methods = res.methods || [];
			smsEnabled = res.smsEnabled === true;
		} catch (e: unknown) {
			loadError = e instanceof Error ? e.message : 'Failed to load contact methods';
			methods = [];
		} finally {
			loading = false;
		}
	}

	function methodSummary(method: ContactMethod): string {
		const config = method.config || {};
		if (method.methodType === 'email') {
			return config.email || 'Account email';
		}
		if (method.methodType === 'slack') {
			try {
				return new URL(config.webhookUrl).host;
			} catch {
				return 'Webhook';
			}
		}
		if (method.methodType === 'pushover') {
			const key = config.userKey || '';
			return key.length > 4 ? `${key.slice(0, 4)}…` : key || '—';
		}
		if (method.methodType === 'telegram') {
			return config.chatId ? `Chat ${config.chatId}` : '—';
		}
		if (method.methodType === 'sms') {
			return config.phoneNumber || '—';
		}
		return '—';
	}

	async function toggleEnabled(method: ContactMethod, enabled: boolean) {
		try {
			await api.put(`/contact-methods/${method.id}`, { enabled });
			toast.success(`Successfully ${enabled ? 'enabled' : 'disabled'} the Contact Method`);
		} catch (e: unknown) {
			toast.error(e instanceof Error ? e.message : 'Failed to update contact method');
		}
		loadMethods();
	}

	async function testMethod(method: ContactMethod) {
		testingId = method.id;
		try {
			await api.post(`/contact-methods/${method.id}/test`, {});
			toast.success('Test notification sent');
		} catch (e: unknown) {
			toast.error(e instanceof Error ? e.message : 'Failed to send test notification');
		} finally {
			testingId = null;
		}
	}

	async function confirmDelete() {
		if (!methodToDelete) return;
		deleting = true;
		deleteError = '';
		try {
			await api.delete(`/contact-methods/${methodToDelete.id}`);
			toast.success('Successfully deleted the Contact Method');
			deleteDialogOpen = false;
			methodToDelete = null;
			loadMethods();
			onMethodsChanged?.();
		} catch (e: unknown) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete contact method';
		} finally {
			deleting = false;
		}
	}
</script>

<InfoCallout>
	When you're paged, Traceway notifies every enabled method. With none configured, your account
	email is used.
</InfoCallout>

{#if loading}
	<div class="flex justify-center py-12"><LoadingCircle size="lg" /></div>
{:else if loadError}
	<ErrorRetryBox message={loadError} onRetry={() => loadMethods()} />
{:else if methods.length === 0}
	<EmptyState
		message="No contact methods yet. Pages fall back to your account email."
		actionLabel="Create your first Contact Method"
		onAction={() => (showCreate = true)}
	/>
{:else}
	<TableContainer>
		<Table.Root>
			<Table.Header>
				<Table.Row class="hover:bg-transparent">
					<Table.Head>Method</Table.Head>
					<Table.Head>Destination</Table.Head>
					<Table.Head>Enabled</Table.Head>
					<Table.Head class="w-[100px] text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each methods as method (method.id)}
					{@const Icon = methodTypeIcons[method.methodType] ?? Bell}
					<Table.Row>
						<Table.Cell class="font-medium">
							<div class="flex items-center gap-2">
								<Icon class="h-4 w-4 text-muted-foreground" />
								{methodTypeLabels[method.methodType] ?? method.methodType}
							</div>
						</Table.Cell>
						<Table.Cell class="text-muted-foreground">
							<div class="flex items-center gap-2">
								{methodSummary(method)}
								{#if isUnsendableSms(method)}
									<Badge
										variant="outline"
										title="This instance has no Twilio credentials configured"
									>
										SMS unavailable
									</Badge>
								{:else if isUnverifiedSms(method)}
									<Badge class="bg-amber-500 text-white">Unverified</Badge>
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={method.enabled}
								onCheckedChange={(checked) => toggleEnabled(method, checked)}
							/>
						</Table.Cell>
						<Table.Cell class="text-right">
							<div class="flex justify-end gap-1">
								{#if isUnverifiedSms(method) && !isUnsendableSms(method)}
									<Button
										variant="ghost"
										size="icon"
										title="Verify"
										onclick={() => openVerify(method)}
									>
										<ShieldCheck class="h-4 w-4" />
									</Button>
								{/if}
								<span
									title={isUnsendableSms(method)
										? 'SMS is unavailable: this instance has no Twilio credentials configured.'
										: isUnverifiedSms(method)
											? 'Verify this phone number before testing it.'
											: undefined}
								>
									<Button
										variant="ghost"
										size="icon"
										title="Test"
										disabled={testingId === method.id ||
											isUnverifiedSms(method) ||
											isUnsendableSms(method)}
										onclick={() => testMethod(method)}
									>
										<Send class="h-4 w-4" />
									</Button>
								</span>
								<Button
									variant="ghost"
									size="icon"
									title="Edit"
									onclick={() => {
										methodToEdit = method;
										showEdit = true;
									}}
								>
									<Pencil class="h-4 w-4" />
								</Button>
								<Button
									variant="ghost"
									size="icon"
									title="Delete"
									onclick={() => {
										deleteError = '';
										methodToDelete = method;
										deleteDialogOpen = true;
									}}
								>
									<Trash2 class="h-4 w-4 text-destructive" />
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</TableContainer>
{/if}

<ContactMethodDialog
	bind:open={showCreate}
	{smsEnabled}
	onSaved={(created) => {
		showCreate = false;
		loadMethods();
		if (created && created.methodType === 'sms' && created.verified === false) {
			openVerify(created, true);
		}
	}}
/>

<ContactMethodDialog
	bind:open={showEdit}
	{smsEnabled}
	method={methodToEdit}
	onSaved={(updated) => {
		showEdit = false;
		methodToEdit = null;
		loadMethods();
		if (updated && updated.methodType === 'sms' && updated.verified === false) {
			openVerify(updated, true);
		}
	}}
/>

<VerifyCodeDialog
	bind:open={showVerify}
	methodId={verifyMethod?.id ?? null}
	phoneLabel={verifyMethod?.config?.phoneNumber ?? 'your phone'}
	codeJustSent={verifyCodeJustSent}
	onVerified={() => loadMethods()}
/>

<ConfirmDeleteDialog
	bind:open={deleteDialogOpen}
	entity="Contact Method"
	description={`Are you sure you want to delete this ${methodTypeLabels[methodToDelete?.methodType ?? ''] ?? ''} contact method? You will no longer be paged through it, and any step of your notification rules that uses it is removed too. To change where it points, edit it instead.`}
	error={deleteError}
	loading={deleting}
	onConfirm={confirmDelete}
/>
