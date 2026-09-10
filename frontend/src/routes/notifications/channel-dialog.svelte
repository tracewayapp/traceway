<script lang="ts">
	import { resolveHref } from '$lib/utils/links';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import * as Select from '$lib/components/ui/select';
	import { Plus, Check, Trash2 } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { SECRET_SENTINEL, type NotificationChannelConfig } from '$lib/types/notifications';

	interface NotificationChannel {
		id: number;
		projectId: string;
		name: string;
		channelType: string;
		config: NotificationChannelConfig;
		hasSecrets?: string[];
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
	let pushoverPriority = $state('0');
	let pushoverRetry = $state(30);
	let pushoverExpire = $state(3600);
	let pushoverCallback = $state('');
	let pushoverSound = $state('');
	let pushoverHtml = $state(false);
	let pushoverTtl = $state(0);
	let telegramBotToken = $state('');
	let telegramChatId = $state('');
	let escalationPolicyId = $state<number | null>(null);
	let escalationPolicies = $state<{ id: number; name: string }[]>([]);
	let escalationPoliciesLoaded = $state(false);
	let agentProfileId = $state<number | null>(null);
	let agentApproval = $state<'ask' | 'auto'>('ask');
	let agentProfiles = $state<{ id: number; name: string; isDefault: boolean }[]>([]);
	let agentProfilesLoaded = $state(false);
	let githubIntegrationId = $state<number | null>(null);
	let codeHostIntegrations = $state<{ id: number; name: string; provider: string }[]>([]);
	let codeHostIntegrationsLoaded = $state(false);
	let slackAppIntegrationId = $state<number | null>(null);
	let slackAppChannel = $state('');
	let chatIntegrations = $state<{ id: number; name: string; provider: string }[]>([]);
	let chatIntegrationsLoaded = $state(false);
	let storedSecrets = $state<string[]>([]);

	const isEditing = $derived(channel !== null);

	function hasStoredSecret(field: string): boolean {
		return isEditing && channelType === channel?.channelType && storedSecrets.includes(field);
	}

	// A credential the API masked is shown as an empty field; leaving it empty
	// sends the sentinel back so the stored value survives the update.
	function secretValue(field: string, typed: string): string {
		return typed || (hasStoredSecret(field) ? SECRET_SENTINEL : '');
	}

	function secretPlaceholder(field: string, fallback: string): string {
		return hasStoredSecret(field) ? 'Secret set, leave blank to keep' : fallback;
	}

	const channelTypeOptions = [
		{ value: 'email', label: 'Email' },
		{ value: 'webhook', label: 'Webhook' },
		{ value: 'slack', label: 'Slack' },
		{ value: 'slack_app', label: 'Slack app (agent)' },
		{ value: 'agent', label: 'Fix agent' },
		{ value: 'github', label: 'GitHub' },
		{ value: 'pushover', label: 'Pushover' },
		{ value: 'telegram', label: 'Telegram' },
		{ value: 'escalation', label: 'Escalation policy' }
	];

	async function loadEscalationPolicies() {
		try {
			const res = await api.get('/escalation-policies', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			escalationPolicies = res.policies || [];
		} catch {
			escalationPolicies = [];
		} finally {
			escalationPoliciesLoaded = true;
		}
	}

	async function loadChatIntegrations() {
		try {
			const res = await api.get('/integrations?kind=chat', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			chatIntegrations = (res.integrations || []).filter(
				(i: { provider: string }) => i.provider === 'slack'
			);
		} catch {
			chatIntegrations = [];
		} finally {
			chatIntegrationsLoaded = true;
		}
	}

	async function loadAgentProfiles() {
		try {
			const res = await api.get('/agent-profiles', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			agentProfiles = res.profiles || [];
		} catch {
			agentProfiles = [];
		} finally {
			agentProfilesLoaded = true;
		}
	}

	async function loadCodeHostIntegrations() {
		try {
			const res = await api.get('/integrations?kind=code_host', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			codeHostIntegrations = (res.integrations || []).filter(
				(i: { provider: string }) => i.provider === 'github'
			);
		} catch {
			codeHostIntegrations = [];
		} finally {
			codeHostIntegrationsLoaded = true;
		}
	}

	$effect(() => {
		if (open && channelType === 'escalation' && !escalationPoliciesLoaded) {
			loadEscalationPolicies();
		}
		if (open && channelType === 'slack_app' && !chatIntegrationsLoaded) {
			loadChatIntegrations();
		}
		if (open && channelType === 'github' && !codeHostIntegrationsLoaded) {
			loadCodeHostIntegrations();
		}
		if (open && channelType === 'agent' && !agentProfilesLoaded) {
			loadAgentProfiles();
		}
	});

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
		pushoverPriority = '0';
		pushoverRetry = 30;
		pushoverExpire = 3600;
		pushoverCallback = '';
		pushoverSound = '';
		pushoverHtml = false;
		pushoverTtl = 0;
		telegramBotToken = '';
		telegramChatId = '';
		escalationPolicyId = null;
		escalationPoliciesLoaded = false;
		slackAppIntegrationId = null;
		slackAppChannel = '';
		chatIntegrationsLoaded = false;
		githubIntegrationId = null;
		codeHostIntegrationsLoaded = false;
		agentProfileId = null;
		agentApproval = 'ask';
		agentProfilesLoaded = false;
		storedSecrets = [];
	}

	function populateFromChannel(ch: NotificationChannel) {
		name = ch.name;
		channelType = ch.channelType;
		storedSecrets = ch.hasSecrets ?? [];
		const config = ch.config || {};

		if (ch.channelType === 'email') {
			emailRecipients = config.recipients?.length ? [...config.recipients] : [''];
		} else if (ch.channelType === 'webhook') {
			webhookUrl = config.url || '';
			webhookMethod = config.method || 'POST';
			webhookSecret = '';
			webhookHeaders = config.headers
				? Object.entries(config.headers).map(([key, value]) => ({
						key,
						value: value as string
					}))
				: [];
		} else if (ch.channelType === 'slack') {
			slackWebhookUrl = '';
			slackChannel = config.channel || '';
			slackUsername = config.username || '';
		} else if (ch.channelType === 'github') {
			githubToken = '';
			githubIntegrationId = config.integrationId ?? null;
			githubOwner = config.owner || '';
			githubRepo = config.repo || '';
			githubLabels = (config.labels || []).join(', ');
		} else if (ch.channelType === 'pushover') {
			pushoverUserKey = '';
			pushoverAppToken = '';
			pushoverDevice = config.device || '';
			pushoverPriority = String(config.priority ?? 0);
			pushoverRetry = config.retry ?? 30;
			pushoverExpire = config.expire ?? 3600;
			pushoverCallback = config.callback || '';
			pushoverSound = config.sound || '';
			pushoverHtml = config.html ?? false;
			pushoverTtl = config.ttl ?? 0;
		} else if (ch.channelType === 'telegram') {
			telegramBotToken = '';
			telegramChatId = config.chatId || '';
		} else if (ch.channelType === 'escalation') {
			escalationPolicyId = config.policyId ?? null;
		} else if (ch.channelType === 'slack_app') {
			slackAppIntegrationId = config.integrationId ?? null;
			slackAppChannel = config.channel || '';
		} else if (ch.channelType === 'agent') {
			agentProfileId = config.profileId ?? null;
			agentApproval = config.approval === 'auto' ? 'auto' : 'ask';
		}
	}

	function buildConfig(): NotificationChannelConfig {
		if (channelType === 'email') {
			return { recipients: emailRecipients.filter((e) => e.trim() !== '') };
		} else if (channelType === 'webhook') {
			const config: NotificationChannelConfig = { url: webhookUrl };
			if (webhookMethod !== 'POST') config.method = webhookMethod;
			const secret = secretValue('secret', webhookSecret);
			if (secret) config.secret = secret;
			const headers: Record<string, string> = {};
			for (const h of webhookHeaders) {
				if (h.key.trim()) headers[h.key.trim()] = h.value;
			}
			if (Object.keys(headers).length > 0) config.headers = headers;
			return config;
		} else if (channelType === 'slack') {
			const config: NotificationChannelConfig = {
				webhookUrl: secretValue('webhookUrl', slackWebhookUrl)
			};
			if (slackChannel) config.channel = slackChannel;
			if (slackUsername) config.username = slackUsername;
			return config;
		} else if (channelType === 'github') {
			const config: NotificationChannelConfig = {
				token: secretValue('token', githubToken),
				owner: githubOwner,
				repo: githubRepo
			};
			if (githubIntegrationId !== null) config.integrationId = githubIntegrationId;
			const labels = githubLabels
				.split(',')
				.map((l) => l.trim())
				.filter((l) => l);
			if (labels.length > 0) config.labels = labels;
			return config;
		} else if (channelType === 'pushover') {
			const config: NotificationChannelConfig = {
				userKey: secretValue('userKey', pushoverUserKey),
				appToken: secretValue('appToken', pushoverAppToken)
			};
			if (pushoverDevice) config.device = pushoverDevice;
			if (pushoverPriority !== '0') config.priority = Number(pushoverPriority);
			if (pushoverPriority === '2') {
				config.retry = Number(pushoverRetry);
				config.expire = Number(pushoverExpire);
				if (pushoverCallback) config.callback = pushoverCallback;
			}
			if (pushoverSound) config.sound = pushoverSound;
			if (pushoverHtml) config.html = pushoverHtml;
			if (Number(pushoverTtl) > 0) config.ttl = Number(pushoverTtl);
			return config;
		} else if (channelType === 'telegram') {
			return {
				botToken: secretValue('botToken', telegramBotToken),
				chatId: telegramChatId
			};
		} else if (channelType === 'escalation') {
			return { policyId: escalationPolicyId };
		} else if (channelType === 'slack_app') {
			return { integrationId: slackAppIntegrationId, channel: slackAppChannel };
		} else if (channelType === 'agent') {
			return { profileId: agentProfileId, approval: agentApproval };
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
	<AlertDialog.Content class="max-h-[90vh] max-w-md overflow-y-auto">
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
			<ErrorAlert {error} />

			<div class="space-y-2">
				<Label for="channel-name">Name</Label>
				<Input id="channel-name" bind:value={name} placeholder="e.g. Team Slack" required />
			</div>

			<div class="space-y-2">
				<Label for="channel-type">Type</Label>
				<Select.Root type="single" bind:value={channelType}>
					<Select.Trigger class="w-full">
						{channelTypeOptions.find((o) => o.value === channelType)?.label || 'Select type'}
					</Select.Trigger>
					<Select.Content>
						{#each channelTypeOptions as option, __index (__index)}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			{#if channelType === 'email'}
				<div class="space-y-2">
					<Label>Recipients</Label>
					{#each emailRecipients as recipient, index (index)}
						<div class="flex gap-2">
							<Input
								type="email"
								value={recipient}
								oninput={(event) =>
									(emailRecipients[index] = (event.currentTarget as HTMLInputElement).value)}
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
						type="password"
						bind:value={webhookSecret}
						placeholder={secretPlaceholder('secret', 'HMAC signing secret')}
					/>
				</div>
				<div class="space-y-2">
					<Label>Headers (optional)</Label>
					{#each webhookHeaders as header, index (index)}
						<div class="flex gap-2">
							<Input bind:value={header.key} placeholder="Header name" class="flex-1" />
							<Input bind:value={header.value} placeholder="Value" class="flex-1" />
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
						type="password"
						bind:value={slackWebhookUrl}
						placeholder={secretPlaceholder('webhookUrl', 'https://hooks.slack.com/services/...')}
						required={!hasStoredSecret('webhookUrl')}
					/>
				</div>
				<div class="space-y-2">
					<Label for="slack-channel">Channel Override (optional)</Label>
					<Input id="slack-channel" bind:value={slackChannel} placeholder="#alerts" />
				</div>
				<div class="space-y-2">
					<Label for="slack-username">Username (optional)</Label>
					<Input id="slack-username" bind:value={slackUsername} placeholder="Traceway" />
				</div>
			{:else if channelType === 'github'}
				{#if codeHostIntegrations.length > 0}
					<div class="space-y-2">
						<Label>Credential</Label>
						<Select.Root
							type="single"
							value={githubIntegrationId !== null ? String(githubIntegrationId) : 'own'}
							onValueChange={(val) => {
								githubIntegrationId = val && val !== 'own' ? Number(val) : null;
							}}
						>
							<Select.Trigger class="w-full">
								{codeHostIntegrations.find((i) => i.id === githubIntegrationId)?.name ??
									'Own token'}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="own">Own token</Select.Item>
								{#each codeHostIntegrations as integration (integration.id)}
									<Select.Item value={String(integration.id)}
										>{integration.name} (integration)</Select.Item
									>
								{/each}
							</Select.Content>
						</Select.Root>
						<p class="text-xs text-muted-foreground">
							An integration creates issues with its own credential; labelling those issues starts
							the fix agent.
						</p>
					</div>
				{/if}
				{#if githubIntegrationId === null}
					<div class="space-y-2">
						<Label for="gh-token">Personal Access Token</Label>
						<Input
							id="gh-token"
							type="password"
							bind:value={githubToken}
							placeholder={secretPlaceholder('token', 'ghp_...')}
							required={!hasStoredSecret('token')}
						/>
					</div>
				{/if}
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
					<Input
						id="po-user-key"
						type="password"
						bind:value={pushoverUserKey}
						placeholder={secretPlaceholder('userKey', 'Your Pushover user key')}
						required={!hasStoredSecret('userKey')}
					/>
				</div>
				<div class="space-y-2">
					<Label for="po-app-token">App Token</Label>
					<Input
						id="po-app-token"
						type="password"
						bind:value={pushoverAppToken}
						placeholder={secretPlaceholder('appToken', 'Your Pushover application token')}
						required={!hasStoredSecret('appToken')}
					/>
				</div>
				<div class="space-y-2">
					<Label for="po-device">Device (optional)</Label>
					<Input
						id="po-device"
						bind:value={pushoverDevice}
						placeholder="Leave empty for all devices"
					/>
				</div>
				<div class="space-y-2">
					<Label for="po-priority">Priority</Label>
					<Select.Root type="single" bind:value={pushoverPriority}>
						<Select.Trigger class="w-full">
							{pushoverPriority === '0'
								? 'Normal'
								: pushoverPriority === '1'
									? 'High'
									: 'Emergency'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="0">Normal</Select.Item>
							<Select.Item value="1">High</Select.Item>
							<Select.Item value="2">Emergency</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
				{#if pushoverPriority === '2'}
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
						<Input
							id="po-callback"
							bind:value={pushoverCallback}
							placeholder="https://hooks.example.com/acknowledged"
						/>
					</div>
				{/if}
				<div class="space-y-2">
					<Label for="po-sound">Sound (optional)</Label>
					<Input
						id="po-sound"
						bind:value={pushoverSound}
						placeholder="e.g. pushover, bike, bugle"
					/>
				</div>
				<div class="flex items-center gap-2">
					<input
						id="po-html"
						type="checkbox"
						bind:checked={pushoverHtml}
						class="h-4 w-4 cursor-pointer"
					/>
					<Label for="po-html" class="cursor-pointer">Enable HTML formatting</Label>
				</div>
				<div class="space-y-2">
					<Label for="po-ttl">Time to Live (seconds, 0 = forever)</Label>
					<Input id="po-ttl" type="number" bind:value={pushoverTtl} min={0} placeholder="0" />
				</div>
			{:else if channelType === 'telegram'}
				<div class="space-y-2">
					<Label for="tg-bot-token">Bot Token</Label>
					<Input
						id="tg-bot-token"
						type="password"
						bind:value={telegramBotToken}
						placeholder={secretPlaceholder('botToken', 'Token from @BotFather')}
						required={!hasStoredSecret('botToken')}
					/>
				</div>
				<div class="space-y-2">
					<Label for="tg-chat-id">Chat ID</Label>
					<Input
						id="tg-chat-id"
						bind:value={telegramChatId}
						placeholder="Destination user or group ID"
						required
					/>
				</div>
			{:else if channelType === 'slack_app'}
				<div class="space-y-2">
					<Label>Slack integration</Label>
					{#if chatIntegrationsLoaded && chatIntegrations.length === 0}
						<p class="text-sm text-muted-foreground">
							No Slack integration yet. An organization admin adds one under
							<a
								{...{ href: resolveHref('/settings') }}
								class="text-blue-600 hover:underline dark:text-blue-400">Settings</a
							>.
						</p>
					{:else}
						<Select.Root
							type="single"
							value={slackAppIntegrationId !== null ? String(slackAppIntegrationId) : undefined}
							onValueChange={(val) => {
								if (val) slackAppIntegrationId = Number(val);
							}}
						>
							<Select.Trigger class="w-full">
								{chatIntegrations.find((i) => i.id === slackAppIntegrationId)?.name ??
									'Select integration'}
							</Select.Trigger>
							<Select.Content>
								{#each chatIntegrations as integration (integration.id)}
									<Select.Item value={String(integration.id)}>{integration.name}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/if}
				</div>
				<div class="space-y-2">
					<Label for="slack-app-channel">Channel ID</Label>
					<Input
						id="slack-app-channel"
						bind:value={slackAppChannel}
						placeholder="C0123456789"
						required
					/>
					<p class="text-xs text-muted-foreground">
						Alerts post here with View, Fix it and Archive buttons; the app has to be invited to the
						channel.
					</p>
				</div>
			{:else if channelType === 'agent'}
				<div class="space-y-2">
					<Label>Agent profile</Label>
					<Select.Root
						type="single"
						value={agentProfileId !== null ? String(agentProfileId) : 'default'}
						onValueChange={(val) => {
							agentProfileId = val && val !== 'default' ? Number(val) : null;
						}}
					>
						<Select.Trigger class="w-full">
							{agentProfiles.find((p) => p.id === agentProfileId)?.name ?? 'Organization default'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="default">Organization default</Select.Item>
							{#each agentProfiles as profile (profile.id)}
								<Select.Item value={String(profile.id)}>{profile.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label>Approval</Label>
					<Select.Root
						type="single"
						value={agentApproval}
						onValueChange={(val) => {
							if (val === 'ask' || val === 'auto') agentApproval = val;
						}}
					>
						<Select.Trigger class="w-full" data-testid="agent-approval">
							{agentApproval === 'auto' ? 'Start right away' : 'Ask first'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="ask">Ask first</Select.Item>
							<Select.Item value="auto">Start right away</Select.Item>
						</Select.Content>
					</Select.Root>
					<p class="text-xs text-muted-foreground">
						Attach this channel to a New Issue or Error Regression rule. Ask first parks the attempt
						on the Agent page (and in Slack when a Slack app is connected) until someone approves
						it. Rules of other types create nothing.
					</p>
				</div>
			{:else if channelType === 'escalation'}
				<div class="space-y-2">
					<Label>Escalation Policy</Label>
					{#if escalationPoliciesLoaded && escalationPolicies.length === 0}
						<p class="text-sm text-muted-foreground">
							No escalation policies yet — create one on the
							<a
								{...{ href: resolveHref('/on-call?tab=policies') }}
								class="text-blue-600 hover:underline dark:text-blue-400">On-Call page</a
							>.
						</p>
					{:else}
						<Select.Root
							type="single"
							value={escalationPolicyId !== null ? String(escalationPolicyId) : undefined}
							onValueChange={(val) => {
								if (val) escalationPolicyId = Number(val);
							}}
						>
							<Select.Trigger class="w-full">
								{escalationPolicies.find((p) => p.id === escalationPolicyId)?.name ??
									'Select policy'}
							</Select.Trigger>
							<Select.Content>
								{#each escalationPolicies as policy (policy.id)}
									<Select.Item value={String(policy.id)}>{policy.name}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/if}
					<p class="text-xs text-muted-foreground">
						Testing an escalation channel opens a real page and notifies the on-call responder.
					</p>
				</div>
			{/if}
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
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
