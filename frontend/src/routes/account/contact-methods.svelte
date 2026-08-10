<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Table from '$lib/components/ui/table';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Switch } from '$lib/components/ui/switch';
	import { Badge } from '$lib/components/ui/badge';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import {
		Plus,
		Trash2,
		Info,
		Send,
		Mail,
		MessageSquare,
		Bell,
		Smartphone,
		ShieldCheck
	} from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { type ContactMethod } from '$lib/state/oncall.svelte';
	import ContactMethodDialog from './contact-method-dialog.svelte';
	import VerifyCodeDialog from './verify-code-dialog.svelte';

	let methods = $state<ContactMethod[]>([]);
	let smsEnabled = $state(false);
	let loading = $state(true);
	let showCreate = $state(false);
	let methodToDelete = $state<ContactMethod | null>(null);
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
		try {
			const res = await api.get('/contact-methods');
			methods = res.methods || [];
			smsEnabled = res.smsEnabled === true;
		} catch {
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
			toast.success(`Successfully ${enabled ? 'enabled' : 'disabled'} the Contact Method`, {
				position: 'top-center'
			});
		} catch (e: unknown) {
			toast.error(e instanceof Error ? e.message : 'Failed to update contact method', {
				position: 'top-center'
			});
		}
		loadMethods();
	}

	async function testMethod(method: ContactMethod) {
		testingId = method.id;
		try {
			await api.post(`/contact-methods/${method.id}/test`, {});
			toast.success('Test notification sent', { position: 'top-center' });
		} catch (e: unknown) {
			toast.error(e instanceof Error ? e.message : 'Failed to send test notification', {
				position: 'top-center'
			});
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
			toast.success('Successfully deleted the Contact Method', { position: 'top-center' });
			methodToDelete = null;
			loadMethods();
		} catch (e: unknown) {
			deleteError = e instanceof Error ? e.message : 'Failed to delete contact method';
		} finally {
			deleting = false;
		}
	}
</script>

<div class="flex items-center justify-between">
	<h2 class="text-xl font-semibold tracking-tight">Contact Methods</h2>
	<Button variant="success" onclick={() => (showCreate = true)}>
		<Plus class="mr-1 h-4 w-4" /> New Contact Method
	</Button>
</div>

<Alert.Root
	class="border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-200"
>
	<Info class="text-blue-600 dark:text-blue-400" />
	<Alert.Description class="text-blue-800 dark:text-blue-300">
		When you're paged, Traceway notifies every enabled method. With none configured, your account
		email is used.
	</Alert.Description>
</Alert.Root>

{#if loading}
	<div class="flex justify-center py-12"><LoadingCircle size="lg" /></div>
{:else if methods.length === 0}
	<div
		class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
	>
		<p class="mb-4">No contact methods yet. Pages fall back to your account email.</p>
		<Button variant="success" onclick={() => (showCreate = true)}>
			<Plus class="mr-1 h-4 w-4" />
			Create your first Contact Method
		</Button>
	</div>
{:else}
	<div class="overflow-hidden rounded-md border">
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
									<Badge variant="outline" title="This instance has no Twilio credentials configured">
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
									title="Delete"
									onclick={() => {
										deleteError = '';
										methodToDelete = method;
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
	</div>
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

<VerifyCodeDialog
	bind:open={showVerify}
	methodId={verifyMethod?.id ?? null}
	phoneLabel={verifyMethod?.config?.phoneNumber ?? 'your phone'}
	codeJustSent={verifyCodeJustSent}
	onVerified={() => loadMethods()}
/>

<AlertDialog.Root
	open={methodToDelete !== null}
	onOpenChange={(open) => {
		if (!open) {
			methodToDelete = null;
			deleteError = '';
		}
	}}
>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Contact Method</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete this {methodTypeLabels[methodToDelete?.methodType ?? ''] ??
					''} contact method? You will no longer be paged through it.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={deleteError} />
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				<Trash2 class="mr-2 h-4 w-4" />
				{deleting ? 'Deleting...' : 'Delete Contact Method'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
