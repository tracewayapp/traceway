<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Check, Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getCurrentUserFromToken, type ContactMethod } from '$lib/state/oncall.svelte';

	interface Props {
		open: boolean;
		smsEnabled: boolean;
		// When set, the dialog edits that method instead of creating one.
		method?: ContactMethod | null;
		onSaved: (created?: ContactMethod) => void;
	}

	let { open = $bindable(), smsEnabled, method = null, onSaved }: Props = $props();

	const isEditing = $derived(method !== null);

	let methodType = $state('email');
	let loading = $state(false);
	let error = $state('');

	let email = $state('');
	let slackWebhookUrl = $state('');
	let slackChannel = $state('');
	let slackUsername = $state('');
	let pushoverUserKey = $state('');
	let pushoverAppToken = $state('');
	let telegramBotToken = $state('');
	let telegramChatId = $state('');
	let smsPhoneNumber = $state('');

	const accountEmail = getCurrentUserFromToken()?.email ?? '';
	const emailPlaceholder = accountEmail
		? `Account email (${accountEmail})`
		: 'Leave empty to use your account email';

	// SMS is only offered when the server has Twilio credentials; without them
	// the instance cannot send anything to a phone.
	const methodTypeOptions = $derived([
		{ value: 'email', label: 'Email' },
		{ value: 'slack', label: 'Slack' },
		{ value: 'pushover', label: 'Pushover' },
		{ value: 'telegram', label: 'Telegram' },
		...(smsEnabled ? [{ value: 'sms', label: 'SMS (Twilio)' }] : [])
	]);

	$effect(() => {
		if (!open) return;
		const editing = method;
		const config = (editing?.config ?? {}) as Record<string, string>;
		methodType = editing?.methodType ?? 'email';
		email = config.email ?? '';
		slackWebhookUrl = config.webhookUrl ?? '';
		slackChannel = config.channel ?? '';
		slackUsername = config.username ?? '';
		pushoverUserKey = config.userKey ?? '';
		pushoverAppToken = config.appToken ?? '';
		telegramBotToken = config.botToken ?? '';
		telegramChatId = config.chatId ?? '';
		smsPhoneNumber = config.phoneNumber ?? '';
		error = '';
	});

	function buildConfig(): Record<string, string> {
		if (methodType === 'email') {
			const config: Record<string, string> = {};
			if (email.trim()) config.email = email.trim();
			return config;
		}
		if (methodType === 'slack') {
			const config: Record<string, string> = { webhookUrl: slackWebhookUrl };
			if (slackChannel) config.channel = slackChannel;
			if (slackUsername) config.username = slackUsername;
			return config;
		}
		if (methodType === 'pushover') {
			return { userKey: pushoverUserKey, appToken: pushoverAppToken };
		}
		if (methodType === 'sms') {
			return { phoneNumber: smsPhoneNumber.trim() };
		}
		return { botToken: telegramBotToken, chatId: telegramChatId };
	}

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			if (method) {
				const updated = await api.put(`/contact-methods/${method.id}`, {
					methodType,
					config: buildConfig()
				});
				toast.success('Successfully updated the Contact Method', { position: 'top-center' });
				onSaved(updated);
			} else {
				const created = await api.post('/contact-methods', { methodType, config: buildConfig() });
				toast.success('Successfully created the Contact Method', { position: 'top-center' });
				onSaved(created);
			}
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save contact method';
		} finally {
			loading = false;
		}
	}
</script>

<AlertDialog.Root {open} onOpenChange={(isOpen) => (open = isOpen)}>
	<AlertDialog.Content class="max-h-[90vh] max-w-md overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title
				>{isEditing ? 'Edit Contact Method' : 'New Contact Method'}</AlertDialog.Title
			>
			<AlertDialog.Description>
				{isEditing
					? "Change where Traceway reaches you when you're paged"
					: "Add a way for Traceway to reach you when you're paged"}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<ErrorAlert {error} />

			<div class="space-y-2">
				<Label>Type</Label>
				<Select.Root type="single" bind:value={methodType}>
					<Select.Trigger class="w-full">
						{methodTypeOptions.find((o) => o.value === methodType)?.label || 'Select type'}
					</Select.Trigger>
					<Select.Content>
						{#each methodTypeOptions as option (option.value)}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			{#if methodType === 'email'}
				<div class="space-y-2">
					<Label for="cm-email">Email (optional)</Label>
					<Input id="cm-email" type="email" bind:value={email} placeholder={emailPlaceholder} />
				</div>
			{:else if methodType === 'slack'}
				<div class="space-y-2">
					<Label for="cm-slack-url">Webhook URL</Label>
					<Input
						id="cm-slack-url"
						bind:value={slackWebhookUrl}
						placeholder="https://hooks.slack.com/services/..."
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="cm-slack-channel">Channel Override (optional)</Label>
					<Input id="cm-slack-channel" bind:value={slackChannel} placeholder="#pages" />
				</div>
				<div class="space-y-2">
					<Label for="cm-slack-username">Username (optional)</Label>
					<Input id="cm-slack-username" bind:value={slackUsername} placeholder="Traceway" />
				</div>
			{:else if methodType === 'pushover'}
				<div class="space-y-2">
					<Label for="cm-po-user-key">User Key</Label>
					<Input
						id="cm-po-user-key"
						bind:value={pushoverUserKey}
						placeholder="Your Pushover user key"
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="cm-po-app-token">App Token</Label>
					<Input
						id="cm-po-app-token"
						type="password"
						bind:value={pushoverAppToken}
						placeholder="Your Pushover application token"
						required
					/>
				</div>
			{:else if methodType === 'telegram'}
				<div class="space-y-2">
					<Label for="cm-tg-bot-token">Bot Token</Label>
					<Input
						id="cm-tg-bot-token"
						type="password"
						bind:value={telegramBotToken}
						placeholder="Token from @BotFather"
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="cm-tg-chat-id">Chat ID</Label>
					<Input
						id="cm-tg-chat-id"
						bind:value={telegramChatId}
						placeholder="Destination user or group ID"
						required
					/>
				</div>
			{:else if methodType === 'sms'}
				<div class="space-y-2">
					<Label for="cm-sms-phone">Phone Number</Label>
					<Input
						id="cm-sms-phone"
						bind:value={smsPhoneNumber}
						placeholder="+12025550123"
						required
					/>
					<p class="text-xs text-muted-foreground">
						International format with country code (E.164)
					</p>
				</div>
			{/if}
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Contact Method'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Contact Method'}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
