<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Plus, Check, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';

	interface NotificationChannel {
		id: number;
		projectId: string;
		name: string;
		channelType: string;
		config: any;
		enabled: boolean;
		createdAt: string;
	}

	interface Props {
		open: boolean;
		channel: NotificationChannel | null;
		onSaved: () => void;
	}

	let { open = $bindable(), channel, onSaved }: Props = $props();

	let name = $state('');
	let channelType = $state('email');
	let loading = $state(false);
	let error = $state('');

	let emailRecipients = $state<string[]>(['']);
	let webhookUrl = $state('');
	let webhookMethod = $state('POST');
	let webhookSecret = $state('');
	let webhookHeaders = $state<{ key: string; value: string }[]>([]);
	let slackWebhookUrl = $state('');
	let slackChannel = $state('');
	let slackUsername = $state('');
	let githubToken = $state('');
	let githubOwner = $state('');
	let githubRepo = $state('');
	let githubLabels = $state('');
	let pushoverUserKey = $state('');
	let pushoverAppToken = $state('');
	let pushoverDevice = $state('');
	let pushoverPriority = $state(0);
	let pushoverRetry = $state(30);
	let pushoverExpire = $state(3600);
	let pushoverCallback = $state('');
	let pushoverSound = $state('');
	let pushoverHtml = $state(false);
	let pushoverTtl = $state(0);

	const isEditing = $derived(channel !== null);

	const channelTypeOptions = [
		{ value: 'email', label: 'Email' },
		{ value: 'webhook', label: 'Webhook' },
		{ value: 'slack', label: 'Slack' },
		{ value: 'github', label: 'GitHub' },
		{ value: 'pushover', label: 'Pushover' }
	];

	function resetForm() {
		name = '';
		channelType = 'email';
		error = '';
		emailRecipients = [''];
		webhookUrl = '';
		webhookMethod = 'POST';
		webhookSecret = '';
		webhookHeaders = [];
		slackWebhookUrl = '';
		slackChannel = '';
		slackUsername = '';
		githubToken = '';
		githubOwner = '';
		githubRepo = '';
		githubLabels = '';
		pushoverUserKey = '';
		pushoverAppToken = '';
		pushoverDevice = '';
		pushoverPriority = 0;
		pushoverRetry = 30;
		pushoverExpire = 3600;
		pushoverCallback = '';
		pushoverSound = '';
		pushoverHtml = false;
		pushoverTtl = 0;
	}

	function populateFromChannel(ch: NotificationChannel) {
		name = ch.name;
		channelType = ch.channelType;
		const config = ch.config || {};

		if (ch.channelType === 'email') {
			emailRecipients = config.recipients?.length ? [...config.recipients] : [''];
		} else if (ch.channelType === 'webhook') {
			webhookUrl = config.url || '';
			webhookMethod = config.method || 'POST';
			webhookSecret = config.secret || '';
			webhookHeaders = config.headers
				? Object.entries(config.headers).map(([key, value]) => ({
						key,
						value: value as string
					}))
				: [];
		} else if (ch.channelType === 'slack') {
			slackWebhookUrl = config.webhookUrl || '';
			slackChannel = config.channel || '';
			slackUsername = config.username || '';
		} else if (ch.channelType === 'github') {
			githubToken = config.token || '';
			githubOwner = config.owner || '';
			githubRepo = config.repo || '';
			githubLabels = (config.labels || []).join(', ');
		} else if (ch.channelType === 'pushover') {
			pushoverUserKey = config.userKey || '';
			pushoverAppToken = config.appToken || '';
			pushoverDevice = config.device || '';
			pushoverPriority = config.priority ?? 0;
			pushoverRetry = config.retry ?? 30;
			pushoverExpire = config.expire ?? 3600;
			pushoverCallback = config.callback || '';
			pushoverSound = config.sound || '';
			pushoverHtml = config.html ?? false;
			pushoverTtl = config.ttl ?? 0;
		}
	}

	function buildConfig(): any {
		if (channelType === 'email') {
			return { recipients: emailRecipients.filter((e) => e.trim() !== '') };
		} else if (channelType === 'webhook') {
			const config: any = { url: webhookUrl };
			if (webhookMethod !== 'POST') config.method = webhookMethod;
			if (webhookSecret) config.secret = webhookSecret;
			const headers: Record<string, string> = {};
			for (const h of webhookHeaders) {
				if (h.key.trim()) headers[h.key.trim()] = h.value;
			}
			if (Object.keys(headers).length > 0) config.headers = headers;
			return config;
		} else if (channelType === 'slack') {
			const config: any = { webhookUrl: slackWebhookUrl };
			if (slackChannel) config.channel = slackChannel;
			if (slackUsername) config.username = slackUsername;
			return config;
		} else if (channelType === 'github') {
			const config: any = {
				token: githubToken,
				owner: githubOwner,
				repo: githubRepo
			};
			const labels = githubLabels
				.split(',')
				.map((l) => l.trim())
				.filter((l) => l);
			if (labels.length > 0) config.labels = labels;
			return config;
		} else if (channelType === 'pushover') {
			const config: any = {
				userKey: pushoverUserKey,
				appToken: pushoverAppToken
			};
			if (pushoverDevice) config.device = pushoverDevice;
			if (pushoverPriority) config.priority = pushoverPriority;
			if (pushoverPriority === 2) {
				config.retry = pushoverRetry;
				config.expire = pushoverExpire;
				if (pushoverCallback) config.callback = pushoverCallback;
			}
			if (pushoverSound) config.sound = pushoverSound;
			if (pushoverHtml) config.html = pushoverHtml;
			if (pushoverTtl > 0) config.ttl = pushoverTtl;
			return config;
		}
		return {};
	}

	function addEmailRecipient() {
		emailRecipients = [...emailRecipients, ''];
	}

	function removeEmailRecipient(index: number) {
		emailRecipients = emailRecipients.filter((_, i) => i !== index);
		if (emailRecipients.length === 0) emailRecipients = [''];
	}

	function addWebhookHeader() {
		webhookHeaders = [...webhookHeaders, { key: '', value: '' }];
	}

	function removeWebhookHeader(index: number) {
		webhookHeaders = webhookHeaders.filter((_, i) => i !== index);
	}

	async function handleSubmit() {
		loading = true;
		error = '';

		try {
			const body = {
				name,
				channelType,
				config: buildConfig()
			};

			if (isEditing) {
				await api.put(`/notification-channels/${channel!.id}`, body, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Channel', { position: 'top-center' });
			} else {
				await api.post('/notification-channels', body, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully created the Channel', { position: 'top-center' });
			}
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save channel';
		} finally {
			loading = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		if (!isOpen) {
			resetForm();
		} else if (channel) {
			populateFromChannel(channel);
		} else {
			resetForm();
		}
		open = isOpen;
	}

	$effect(() => {
		if (open && channel) {
			populateFromChannel(channel);
		} else if (open && !channel) {
			resetForm();
		}
	});
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
	<AlertDialog.Content class="max-w-md max-h-[90vh] overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Channel' : 'New Channel'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the notification channel configuration'
					: 'Configure a new notification channel'}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<div class="space-y-2">
				<Label for="channel-name">Name</Label>
				<Input id="channel-name" bind:value={name} placeholder="e.g. Team Slack" required />
			</div>

			<div class="space-y-2">
				<Label for="channel-type">Type</Label>
				<Select.Root type="single" bind:value={channelType}>
					<Select.Trigger class="w-full">
						{channelTypeOptions.find((o) => o.value === channelType)?.label ||
							'Select type'}
					</Select.Trigger>
					<Select.Content>
						{#each channelTypeOptions as option}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			{#if channelType === 'email'}
				<div class="space-y-2">
					<Label>Recipients</Label>
					{#each emailRecipients as _, index}
						<div class="flex gap-2">
							<Input
								type="email"
								bind:value={emailRecipients[index]}
								placeholder="email@example.com"
							/>
							{#if emailRecipients.length > 1}
								<Button
									variant="ghost"
									size="icon"
									type="button"
									onclick={() => removeEmailRecipient(index)}
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							{/if}
						</div>
					{/each}
					{#if emailRecipients.length < 10}
						<Button variant="outline" size="sm" type="button" onclick={addEmailRecipient}>
							<Plus class="mr-1 h-3 w-3" /> Add Recipient
						</Button>
					{/if}
				</div>
			{:else if channelType === 'webhook'}
				<div class="space-y-2">
					<Label for="webhook-url">URL</Label>
					<Input
						id="webhook-url"
						bind:value={webhookUrl}
						placeholder="https://example.com/webhook"
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="webhook-method">Method</Label>
					<Select.Root type="single" bind:value={webhookMethod}>
						<Select.Trigger class="w-full">
							{webhookMethod}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="POST">POST</Select.Item>
							<Select.Item value="PUT">PUT</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label for="webhook-secret">Secret (optional)</Label>
					<Input
						id="webhook-secret"
						bind:value={webhookSecret}
						placeholder="HMAC signing secret"
					/>
				</div>
				<div class="space-y-2">
					<Label>Headers (optional)</Label>
					{#each webhookHeaders as _, index}
						<div class="flex gap-2">
							<Input
								bind:value={webhookHeaders[index].key}
								placeholder="Header name"
								class="flex-1"
							/>
							<Input
								bind:value={webhookHeaders[index].value}
								placeholder="Value"
								class="flex-1"
							/>
							<Button
								variant="ghost"
								size="icon"
								type="button"
								onclick={() => removeWebhookHeader(index)}
							>
								<Trash2 class="h-4 w-4" />
							</Button>
						</div>
					{/each}
					<Button variant="outline" size="sm" type="button" onclick={addWebhookHeader}>
						<Plus class="mr-1 h-3 w-3" /> Add Header
					</Button>
				</div>
			{:else if channelType === 'slack'}
				<div class="space-y-2">
					<Label for="slack-url">Webhook URL</Label>
					<Input
						id="slack-url"
						bind:value={slackWebhookUrl}
						placeholder="https://hooks.slack.com/services/..."
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="slack-channel">Channel Override (optional)</Label>
					<Input
						id="slack-channel"
						bind:value={slackChannel}
						placeholder="#alerts"
					/>
				</div>
				<div class="space-y-2">
					<Label for="slack-username">Username (optional)</Label>
					<Input
						id="slack-username"
						bind:value={slackUsername}
						placeholder="Traceway"
					/>
				</div>
			{:else if channelType === 'github'}
				<div class="space-y-2">
					<Label for="gh-token">Personal Access Token</Label>
					<Input
						id="gh-token"
						type="password"
						bind:value={githubToken}
						placeholder="ghp_..."
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="gh-owner">Repository Owner</Label>
					<Input id="gh-owner" bind:value={githubOwner} placeholder="owner" required />
				</div>
				<div class="space-y-2">
					<Label for="gh-repo">Repository Name</Label>
					<Input id="gh-repo" bind:value={githubRepo} placeholder="repo" required />
				</div>
				<div class="space-y-2">
					<Label for="gh-labels">Labels (optional, comma-separated)</Label>
					<Input id="gh-labels" bind:value={githubLabels} placeholder="bug, traceway" />
				</div>
			{:else if channelType === 'pushover'}
				<div class="space-y-2">
					<Label for="po-user-key">User Key</Label>
					<Input id="po-user-key" bind:value={pushoverUserKey} placeholder="Your Pushover user key" required />
				</div>
				<div class="space-y-2">
					<Label for="po-app-token">App Token</Label>
					<Input id="po-app-token" type="password" bind:value={pushoverAppToken} placeholder="Your Pushover application token" required />
				</div>
				<div class="space-y-2">
					<Label for="po-device">Device (optional)</Label>
					<Input id="po-device" bind:value={pushoverDevice} placeholder="Leave empty for all devices" />
				</div>
				<div class="space-y-2">
					<Label for="po-priority">Priority</Label>
					<Select.Root type="single" bind:value={pushoverPriority}>
						<Select.Trigger class="w-full">
							{pushoverPriority === 0 ? 'Normal' : pushoverPriority === 1 ? 'High' : 'Emergency'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value={0}>Normal</Select.Item>
							<Select.Item value={1}>High</Select.Item>
							<Select.Item value={2}>Emergency</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
				{#if pushoverPriority === 2}
					<div class="space-y-2">
						<Label for="po-retry">Retry Interval (seconds, min 30)</Label>
						<Input id="po-retry" type="number" bind:value={pushoverRetry} min={30} />
					</div>
					<div class="space-y-2">
						<Label for="po-expire">Expiry (seconds, max 10800)</Label>
						<Input id="po-expire" type="number" bind:value={pushoverExpire} min={1} max={10800} />
					</div>
					<div class="space-y-2">
						<Label for="po-callback">Callback URL (optional)</Label>
						<Input id="po-callback" bind:value={pushoverCallback} placeholder="https://hooks.example.com/acknowledged" />
					</div>
				{/if}
				<div class="space-y-2">
					<Label for="po-sound">Sound (optional)</Label>
					<Input id="po-sound" bind:value={pushoverSound} placeholder="e.g. pushover, bike, bugle" />
				</div>
				<div class="flex items-center gap-2">
					<input id="po-html" type="checkbox" bind:checked={pushoverHtml} class="h-4 w-4" />
					<Label for="po-html">Enable HTML formatting</Label>
				</div>
				<div class="space-y-2">
					<Label for="po-ttl">Time to Live (seconds, 0 = forever)</Label>
					<Input id="po-ttl" type="number" bind:value={pushoverTtl} min={0} placeholder="0" />
				</div>
			{/if}

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{#if loading}
						Updating...
					{:else}
						Update Channel
					{/if}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{#if loading}
						Creating...
					{:else}
						New Channel
					{/if}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
