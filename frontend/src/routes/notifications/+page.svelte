<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { toast } from 'svelte-sonner';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Alert from '$lib/components/ui/alert';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Plus, Pencil, Trash2, Zap, ZapOff, Clock, Send, Info } from '@lucide/svelte';
	import { SearchBar } from '$lib/components/ui/search-bar';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { CalendarDate } from '@internationalized/date';
	import {
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import { calendarDateTimeToLuxon, toUTCISO } from '$lib/utils/formatters';

	import ChannelDialog from './channel-dialog.svelte';
	import RuleDialog from './rule-dialog.svelte';
	import SnoozeDialog from './snooze-dialog.svelte';
	import { ruleTypeLabels } from './rule-types';

	interface NotificationChannel {
		id: number;
		projectId: string;
		name: string;
		channelType: string;
		config: any;
		enabled: boolean;
		createdAt: string;
	}

	interface NotificationRule {
		id: number;
		projectId: string;
		channelId: number;
		name: string;
		ruleType: string;
		config: any;
		enabled: boolean;
		cooldownMinutes: number;
		severity: string;
		snoozedUntil: string | null;
		channelName: string;
		channelType: string;
		createdAt: string;
	}

	interface NotificationHistory {
		ruleId: number;
		ruleType: string;
		ruleName: string;
		channelType: string;
		channelName: string;
		severity: string;
		subject: string;
		body: string;
		status: string;
		errorMessage: string;
		url: string;
		createdAt: string;
	}

	const channelTypeLabels: Record<string, string> = {
		email: 'Email',
		webhook: 'Webhook',
		slack: 'Slack',
		github: 'GitHub',
		pushover: 'Pushover',
		telegram: 'Telegram'
	};

	const tabDescriptions: Record<string, string> = {
		channels:
			'Channels define where your notifications are delivered — such as Email, Slack, Webhooks, GitHub Issues, or Pushover. Create a channel first, then attach it to a rule.',
		rules: 'Rules define when notifications are triggered. Each rule monitors a specific condition and sends an alert through the attached channel when that condition is met.',
		history:
			'A log of all notifications that have been sent, including their status and the rule that triggered them.'
	};

	const activeTab = $derived(page.url.searchParams.get('tab') || 'channels');

	function setTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		if (tab !== 'history') {
			url.searchParams.delete('preset');
			url.searchParams.delete('from');
			url.searchParams.delete('to');
		}
		goto(url.toString(), { replaceState: true, noScroll: true });
		if (tab === 'history') {
			loadHistory(false);
		}
	}

	let channels = $state<NotificationChannel[]>([]);
	let channelsLoading = $state(true);

	let rules = $state<NotificationRule[]>([]);
	let rulesLoading = $state(true);

	let history = $state<NotificationHistory[]>([]);
	let historyLoading = $state(true);
	let historyPage = $state(1);
	let historyPageSize = $state(25);
	let historyTotal = $state(0);
	let historyTotalPages = $state(0);
	let searchQuery = $state('');

	const timezone = $derived(getTimezone());

	function parseHistoryUrlParams() {
		if (!browser) return { preset: '7d', from: null, to: null };
		return parseTimeRangeFromUrl(timezone, '7d');
	}

	const initialUrlParams = parseHistoryUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	function getFromDateTimeUTC(): string {
		const [hour, minute] = fromTime.split(':').map(Number);
		const luxonDt = calendarDateTimeToLuxon(
			{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
			timezone
		);
		return toUTCISO(luxonDt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = toTime.split(':').map(Number);
		const luxonDt = calendarDateTimeToLuxon(
			{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
			timezone
		).endOf('minute');
		return toUTCISO(luxonDt);
	}

	function updateHistoryUrl(pushToHistory = true) {
		const params: Record<string, string | null | undefined> = {};
		params.tab = 'history';
		if (selectedPreset) {
			params.preset = selectedPreset;
		} else {
			params.from = getFromDateTimeUTC();
			params.to = getToDateTimeUTC();
		}
		updateUrl(params, { pushToHistory });
	}

	let channelDialogOpen = $state(false);
	let editingChannel = $state<NotificationChannel | null>(null);
	let ruleDialogOpen = $state(false);
	let editingRule = $state<NotificationRule | null>(null);
	let snoozeDialogOpen = $state(false);
	let snoozeRuleId = $state<number | null>(null);

	let showDeleteChannelDialog = $state(false);
	let deletingChannel = $state<NotificationChannel | null>(null);

	let showDeleteRuleDialog = $state(false);
	let deletingRule = $state<NotificationRule | null>(null);

	let showToggleRuleDialog = $state(false);
	let togglingRule = $state<NotificationRule | null>(null);
	let togglingLoading = $state(false);

	let showTestChannelDialog = $state(false);
	let testingChannel = $state<NotificationChannel | null>(null);
	let testingLoading = $state(false);
	let testError = $state('');

	async function loadChannels() {
		channelsLoading = true;
		try {
			const res = await api.get('/notification-channels', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			channels = res.channels || [];
		} catch {
			channels = [];
		} finally {
			channelsLoading = false;
		}
	}

	async function loadRules() {
		rulesLoading = true;
		try {
			const res = await api.get('/notification-rules', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			rules = res.rules || [];
		} catch {
			rules = [];
		} finally {
			rulesLoading = false;
		}
	}

	async function loadHistory(pushToHistory = true) {
		historyLoading = true;

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		updateHistoryUrl(pushToHistory);

		try {
			const res = await api.post(
				'/notification-history',
				{
					pagination: { page: historyPage, pageSize: historyPageSize },
					search: searchQuery.trim(),
					fromDate: getFromDateTimeUTC(),
					toDate: getToDateTimeUTC()
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			history = res.data || [];
			historyTotal = res.pagination?.total || 0;
			historyTotalPages = res.pagination?.totalPages || 0;
		} catch {
			history = [];
		} finally {
			historyLoading = false;
		}
	}

	function handleHistorySearch() {
		historyPage = 1;
		loadHistory(true);
	}

	function openDeleteChannel(channel: NotificationChannel) {
		deletingChannel = channel;
		showDeleteChannelDialog = true;
	}

	async function deleteChannel() {
		if (!deletingChannel) return;
		try {
			await api.delete(`/notification-channels/${deletingChannel.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Channel', { position: 'top-center' });
			showDeleteChannelDialog = false;
			deletingChannel = null;
			loadChannels();
			loadRules();
		} catch {
			toast.error('Failed to delete channel', { position: 'top-center' });
		}
	}

	function openTestChannel(channel: NotificationChannel) {
		testingChannel = channel;
		testError = '';
		showTestChannelDialog = true;
	}

	async function confirmTestChannel() {
		if (!testingChannel) return;
		testingLoading = true;
		testError = '';
		try {
			await api.post(
				`/notification-channels/${testingChannel.id}/test`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Test notification sent', { position: 'top-center' });
			showTestChannelDialog = false;
			testingChannel = null;
		} catch (e: any) {
			if (e.status === 422) {
				testError = e.message || 'Validation failed';
			} else {
				toast.error('An unexpected error has occurred', { position: 'top-center' });
				showTestChannelDialog = false;
				testingChannel = null;
			}
		} finally {
			testingLoading = false;
		}
	}

	function openDeleteRule(rule: NotificationRule) {
		deletingRule = rule;
		showDeleteRuleDialog = true;
	}

	async function deleteRule() {
		if (!deletingRule) return;
		try {
			await api.delete(`/notification-rules/${deletingRule.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Rule', { position: 'top-center' });
			showDeleteRuleDialog = false;
			deletingRule = null;
			loadRules();
		} catch {
			toast.error('Failed to delete rule', { position: 'top-center' });
		}
	}

	function openToggleRule(rule: NotificationRule) {
		togglingRule = rule;
		showToggleRuleDialog = true;
	}

	async function confirmToggleRule() {
		if (!togglingRule) return;
		togglingLoading = true;
		try {
			const action = togglingRule.enabled ? 'disabled' : 'enabled';
			await api.post(
				`/notification-rules/${togglingRule.id}/toggle`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success(`Successfully ${action} the Rule`, { position: 'top-center' });
			showToggleRuleDialog = false;
			togglingRule = null;
			loadRules();
		} catch {
			toast.error('Failed to toggle rule', { position: 'top-center' });
		} finally {
			togglingLoading = false;
		}
	}

	function openSnooze(id: number) {
		snoozeRuleId = id;
		snoozeDialogOpen = true;
	}

	function openEditChannel(channel: NotificationChannel) {
		editingChannel = channel;
		channelDialogOpen = true;
	}

	function openNewChannel() {
		editingChannel = null;
		channelDialogOpen = true;
	}

	function openEditRule(rule: NotificationRule) {
		editingRule = rule;
		ruleDialogOpen = true;
	}

	function openNewRule() {
		editingRule = null;
		ruleDialogOpen = true;
	}

	function formatDate(dateStr: string) {
		const date = new Date(dateStr);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const minutes = Math.floor(diff / 60000);
		if (minutes < 1) return 'just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	function isSnoozed(rule: NotificationRule) {
		return rule.snoozedUntil && new Date(rule.snoozedUntil) > new Date();
	}

	function handleHistoryPageChange(newPage: number) {
		historyPage = newPage;
		loadHistory(true);
	}

	function handleHistoryPageSizeChange(newSize: number) {
		historyPageSize = newSize;
		historyPage = 1;
		loadHistory(true);
	}

	function handleTimeRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		fromDate = from.date;
		toDate = to.date;
		fromTime = from.time;
		toTime = to.time;
		selectedPreset = preset;
		historyPage = 1;
		loadHistory(true);
	}

	function handlePopState() {
		const urlParams = parseHistoryUrlParams();
		const range = getResolvedTimeRange(urlParams, timezone);
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toTime = dateToTimeString(range.to, timezone);
		historyPage = 1;
		loadHistory(false);
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadChannels();
		loadRules();
		if (activeTab === 'history') {
			loadHistory(false);
		}
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-2">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">Alerts</h1>
	</div>

	<div class="flex items-center justify-between">
		<Tabs.Root value={activeTab} onValueChange={(v) => { if (v) setTab(v); }}>
			<Tabs.List>
				<Tabs.Trigger value="channels">Channels</Tabs.Trigger>
				<Tabs.Trigger value="rules">Rules</Tabs.Trigger>
				<Tabs.Trigger value="history">History</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
		{#if activeTab === 'channels'}
			<Button size="sm" onclick={openNewChannel}>
				<Plus class="mr-1 h-4 w-4" /> New Channel
			</Button>
		{:else if activeTab === 'rules'}
			<Button size="sm" onclick={openNewRule}>
				<Plus class="mr-1 h-4 w-4" /> New Rule
			</Button>
		{/if}
	</div>

	<Alert.Root class="bg-blue-50 border-blue-200 text-blue-900 dark:bg-blue-950/50 dark:border-blue-800 dark:text-blue-200">
		<Info class="text-blue-600 dark:text-blue-400" />
		<Alert.Description class="text-blue-800 dark:text-blue-300">{tabDescriptions[activeTab]}</Alert.Description>
	</Alert.Root>

	{#if activeTab === 'channels'}
		{#if channelsLoading}
			<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if channels.length === 0}
			<div
				class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
			>
				<p class="mb-4">No channels yet. Create one to get started.</p>
				<Button onclick={openNewChannel}>
					<Plus class="mr-1 h-4 w-4" />
					Create your first Channel
				</Button>
			</div>
		{:else}
			<div class="overflow-hidden rounded-md border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Name</Table.Head>
							<Table.Head>Type</Table.Head>
							<Table.Head>Enabled</Table.Head>
							<Table.Head>Created</Table.Head>
							<Table.Head class="text-right">Actions</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each channels as channel}
							<Table.Row>
								<Table.Cell class="font-medium">{channel.name}</Table.Cell>
								<Table.Cell>
									<Badge variant="outline"
										>{channelTypeLabels[channel.channelType] ||
											channel.channelType}</Badge
									>
								</Table.Cell>
								<Table.Cell>
									{#if channel.enabled}
										<Badge variant="default" class="bg-green-600">On</Badge>
									{:else}
										<Badge variant="secondary">Off</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-muted-foreground"
									>{formatDate(channel.createdAt)}</Table.Cell
								>
								<Table.Cell class="text-right">
									<div class="flex justify-end gap-1">
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openTestChannel(channel)}
											title="Test"
										>
											<Send class="h-4 w-4" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openEditChannel(channel)}
											title="Edit"
										>
											<Pencil class="h-4 w-4" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openDeleteChannel(channel)}
											title="Delete"
										>
											<Trash2 class="h-4 w-4" />
										</Button>
									</div>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/if}
	{:else if activeTab === 'rules'}
		{#if rulesLoading}
			<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if rules.length === 0}
			<div
				class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
			>
				<p class="mb-4">No rules yet. Create one to get started.</p>
				<Button onclick={openNewRule}>
					<Plus class="mr-1 h-4 w-4" />
					Create your first Rule
				</Button>
			</div>
		{:else}
			<div class="overflow-hidden rounded-md border">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Name</Table.Head>
							<Table.Head>Type</Table.Head>
							<Table.Head>Channel</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head class="text-right">Actions</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each rules as rule}
							<Table.Row>
								<Table.Cell class="font-medium">{rule.name}</Table.Cell>
								<Table.Cell
									>{ruleTypeLabels[rule.ruleType] || rule.ruleType}</Table.Cell
								>
								<Table.Cell>
									<Badge variant="outline">{rule.channelName}</Badge>
								</Table.Cell>
								<Table.Cell>
									{#if isSnoozed(rule)}
										<Badge variant="secondary">Snoozed</Badge>
									{:else if rule.enabled}
										<Badge variant="default" class="bg-green-600">On</Badge>
									{:else}
										<Badge variant="secondary">Off</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-right">
									<div class="flex justify-end gap-1">
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openSnooze(rule.id)}
											title="Snooze"
										>
											<Clock class="h-4 w-4" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openToggleRule(rule)}
											title={rule.enabled ? 'Disable' : 'Enable'}
										>
											{#if rule.enabled}
												<ZapOff class="h-4 w-4" />
											{:else}
												<Zap class="h-4 w-4" />
											{/if}
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openEditRule(rule)}
											title="Edit"
										>
											<Pencil class="h-4 w-4" />
										</Button>
										<Button
											variant="ghost"
											size="icon"
											onclick={() => openDeleteRule(rule)}
											title="Delete"
										>
											<Trash2 class="h-4 w-4" />
										</Button>
									</div>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/if}
	{:else if activeTab === 'history'}
		<div class="flex flex-col gap-2 pt-2 sm:flex-row sm:items-center sm:justify-between">
			<SearchBar
				placeholder="Search Historic Alerts..."
				bind:value={searchQuery}
				onSearch={handleHistorySearch}
				disabled={historyLoading}
			/>
			<div class="w-full sm:w-auto">
				<TimeRangePicker
					bind:fromDate
					bind:toDate
					bind:fromTime
					bind:toTime
					bind:preset={selectedPreset}
					onApply={handleTimeRangeChange}
				/>
			</div>
		</div>

		<div class="overflow-hidden rounded-md border">
			<Table.Root>
				{#if historyLoading}
					<Table.Body>
						<Table.Row>
							<Table.Cell colspan={6} class="h-48">
								<div class="flex h-full items-center justify-center">
									<LoadingCircle size="xlg" />
								</div>
							</Table.Cell>
						</Table.Row>
					</Table.Body>
				{:else if history.length === 0}
					<Table.Body>
						<TableEmptyState colspan={6} message="No Historic Alerts found." />
					</Table.Body>
				{:else}
					<Table.Header>
						<Table.Row>
							<Table.Head>Severity</Table.Head>
							<Table.Head>Rule</Table.Head>
							<Table.Head>Subject</Table.Head>
							<Table.Head>Channel</Table.Head>
							<Table.Head>Status</Table.Head>
							<Table.Head>Sent At</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each history as item}
							<Table.Row>
								<Table.Cell>
									{#if item.severity === 'critical'}
										<Badge variant="destructive">Critical</Badge>
									{:else if item.severity === 'warning'}
										<Badge class="bg-amber-500 text-white">Warning</Badge>
									{:else}
										<Badge variant="secondary">Info</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell class="font-medium">{item.ruleName}</Table.Cell>
								<Table.Cell class="max-w-xs truncate">
									{#if item.url}
										<a href={item.url} class="text-blue-600 hover:underline dark:text-blue-400">{item.subject}</a>
									{:else}
										{item.subject}
									{/if}
								</Table.Cell>
								<Table.Cell>{item.channelName}</Table.Cell>
								<Table.Cell>
									{#if item.status === 'sent'}
										<Badge class="bg-green-600 text-white">Sent</Badge>
									{:else if item.status === 'failed'}
										<Badge variant="destructive">Failed</Badge>
									{:else}
										<Badge variant="secondary">Skipped</Badge>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-muted-foreground"
									>{formatDate(item.createdAt)}</Table.Cell
								>
							</Table.Row>
						{/each}
					</Table.Body>
				{/if}
			</Table.Root>
		</div>

		<PaginationFooter
			currentPage={historyPage}
			totalPages={historyTotalPages}
			pageSize={historyPageSize}
			totalItems={historyTotal}
			onPageChange={handleHistoryPageChange}
			onPageSizeChange={handleHistoryPageSizeChange}
			itemLabel="notification"
		/>
	{/if}
</div>

<ChannelDialog
	bind:open={channelDialogOpen}
	channel={editingChannel}
	onSaved={() => {
		loadChannels();
		channelDialogOpen = false;
	}}
/>

<RuleDialog
	bind:open={ruleDialogOpen}
	rule={editingRule}
	{channels}
	onSaved={() => {
		loadRules();
		ruleDialogOpen = false;
	}}
/>

<SnoozeDialog
	bind:open={snoozeDialogOpen}
	ruleId={snoozeRuleId}
	onSaved={() => {
		loadRules();
		snoozeDialogOpen = false;
	}}
/>

<AlertDialog.Root bind:open={showDeleteChannelDialog}>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Channel</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{deletingChannel?.name}"? This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showDeleteChannelDialog = false)}>Cancel</Button>
			<Button variant="destructive" onclick={deleteChannel}>Delete</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showDeleteRuleDialog}>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Rule</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{deletingRule?.name}"? This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showDeleteRuleDialog = false)}>Cancel</Button>
			<Button variant="destructive" onclick={deleteRule}>Delete</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showToggleRuleDialog}>
	<AlertDialog.Content
		interactOutsideBehavior={togglingLoading ? 'ignore' : 'close'}
		escapeKeydownBehavior={togglingLoading ? 'ignore' : 'close'}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>{togglingRule?.enabled ? 'Disable' : 'Enable'} Rule</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to {togglingRule?.enabled ? 'disable' : 'enable'} "{togglingRule?.name}"?
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showToggleRuleDialog = false)} disabled={togglingLoading}>Cancel</Button>
			<Button onclick={confirmToggleRule} disabled={togglingLoading}>
				{togglingRule?.enabled ? 'Disable' : 'Enable'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root bind:open={showTestChannelDialog}>
	<AlertDialog.Content
		interactOutsideBehavior={testingLoading ? 'ignore' : 'close'}
		escapeKeydownBehavior={testingLoading ? 'ignore' : 'close'}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>Test Channel</AlertDialog.Title>
			<AlertDialog.Description>
				Send a test notification to "{testingChannel?.name}"? This will deliver a test message through the configured channel.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if testError}
			<p class="text-sm text-destructive">{testError}</p>
		{/if}
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showTestChannelDialog = false)} disabled={testingLoading}>Cancel</Button>
			<Button onclick={confirmTestChannel} disabled={testingLoading}>
				<Send class="h-4 w-4" /> {testingLoading ? 'Sending...' : 'Send Test'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
