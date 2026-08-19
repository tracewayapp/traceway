<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { resolveHref } from '$lib/utils/links';
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { toast } from 'svelte-sonner';
	import * as Table from '$lib/components/ui/table';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import PageTabs from '$lib/components/traceway/page-tabs.svelte';
	import InfoCallout from '$lib/components/traceway/info-callout.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import ErrorRetryBox from '$lib/components/traceway/error-retry-box.svelte';
	import ConfirmDeleteDialog from '$lib/components/traceway/confirm-delete-dialog.svelte';
	import PageBadges from '$lib/components/traceway/page-badges.svelte';
	import { Pencil, Trash2, Zap, ZapOff, Clock, Send } from '@lucide/svelte';
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
		updateUrl,
		setTabParam
	} from '$lib/utils/url-params';
	import { calendarDateTimeToLuxon, toUTCISO } from '$lib/utils/formatters';

	import ChannelDialog from './channel-dialog.svelte';
	import RuleDialog from './rule-dialog.svelte';
	import SnoozeDialog from './snooze-dialog.svelte';
	import { ruleTypeLabels } from './rule-types';
	import type { NotificationChannelConfig, NotificationRuleConfig } from '$lib/types/notifications';

	interface NotificationChannel {
		id: number;
		projectId: string;
		name: string;
		channelType: string;
		config: NotificationChannelConfig;
		enabled: boolean;
		createdAt: string;
	}

	interface NotificationRule {
		id: number;
		projectId: string;
		channelId: number;
		name: string;
		ruleType: string;
		config: NotificationRuleConfig;
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
		telegram: 'Telegram',
		escalation: 'Escalation policy'
	};

	const tabDescriptions: Record<string, string> = {
		channels:
			'Channels define where your notifications are delivered, such as Email, Slack, Webhooks, GitHub Issues, or Pushover. Create a channel first, then attach it to a rule.',
		rules:
			'Rules define when notifications are triggered. Each rule monitors a specific condition and sends an alert through the attached channel when that condition is met.',
		history:
			'A log of all notifications that have been sent, including their status and the rule that triggered them.'
	};

	const TABS = [
		{ value: 'channels', label: 'Channels' },
		{ value: 'rules', label: 'Rules' },
		{ value: 'history', label: 'History' }
	];

	const activeTab = $derived(page.url.searchParams.get('tab') || 'channels');

	function setTab(tab: string) {
		setTabParam(tab, { clear: tab !== 'history' ? ['preset', 'from', 'to'] : [] });
		if (tab === 'history') {
			loadHistory(false);
		}
	}

	let channels = $state<NotificationChannel[]>([]);
	let channelsLoading = $state(true);
	let channelsError = $state('');

	let rules = $state<NotificationRule[]>([]);
	let rulesLoading = $state(true);
	let rulesError = $state('');

	let history = $state<NotificationHistory[]>([]);
	let historyLoading = $state(true);
	let historyError = $state('');
	let historyPage = $state(1);
	let historyPageSize = $state(25);
	let historyTotal = $state(0);
	let historyTotalPages = $state(0);
	let searchQuery = $state('');

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	function parseHistoryUrlParams() {
		if (!browser) return { preset: '7d', from: null, to: null };
		return parseTimeRangeFromUrl(timezone, '7d');
	}

	const initialUrlParams = parseHistoryUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

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
	let deleteChannelLoading = $state(false);

	let showDeleteRuleDialog = $state(false);
	let deletingRule = $state<NotificationRule | null>(null);
	let deleteRuleLoading = $state(false);

	let showToggleRuleDialog = $state(false);
	let togglingRule = $state<NotificationRule | null>(null);
	let togglingLoading = $state(false);

	let showTestChannelDialog = $state(false);
	let testingChannel = $state<NotificationChannel | null>(null);
	let testingLoading = $state(false);
	let testError = $state('');

	async function loadChannels() {
		channelsLoading = true;
		channelsError = '';
		try {
			const res = await api.get('/notification-channels', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			channels = res.channels || [];
		} catch (e: unknown) {
			channels = [];
			channelsError = e instanceof Error ? getErrorMessage(e) : 'Failed to load channels';
		} finally {
			channelsLoading = false;
		}
	}

	async function loadRules() {
		rulesLoading = true;
		rulesError = '';
		try {
			const res = await api.get('/notification-rules', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			rules = res.rules || [];
		} catch (e: unknown) {
			rules = [];
			rulesError = e instanceof Error ? getErrorMessage(e) : 'Failed to load rules';
		} finally {
			rulesLoading = false;
		}
	}

	async function loadHistory(pushToHistory = true) {
		historyLoading = true;
		historyError = '';

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
		} catch (e: unknown) {
			history = [];
			historyTotal = 0;
			historyTotalPages = 0;
			historyError = e instanceof Error ? getErrorMessage(e) : 'Failed to load history';
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
		deleteChannelLoading = true;
		try {
			await api.delete(`/notification-channels/${deletingChannel.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Channel');
			showDeleteChannelDialog = false;
			deletingChannel = null;
			loadChannels();
			loadRules();
		} catch {
			toast.error('Failed to delete channel');
		} finally {
			deleteChannelLoading = false;
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
			toast.success('Test notification sent');
			showTestChannelDialog = false;
			testingChannel = null;
		} catch (e) {
			if (getErrorStatus(e) === 422) {
				testError = getErrorMessage(e) || 'Validation failed';
			} else {
				toast.error('An unexpected error has occurred');
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
		deleteRuleLoading = true;
		try {
			await api.delete(`/notification-rules/${deletingRule.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			toast.success('Successfully deleted the Rule');
			showDeleteRuleDialog = false;
			deletingRule = null;
			loadRules();
		} catch {
			toast.error('Failed to delete rule');
		} finally {
			deleteRuleLoading = false;
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
			toast.success(`Successfully ${action} the Rule`);
			showToggleRuleDialog = false;
			togglingRule = null;
			loadRules();
		} catch {
			toast.error('Failed to toggle rule');
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

	const newAction = $derived.by(() => {
		if (activeTab === 'channels') return { label: 'New Channel', onclick: openNewChannel };
		if (activeTab === 'rules') return { label: 'New Rule', onclick: openNewRule };
		return null;
	});

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

<div class="space-y-4">
	<PageHeader title="Alerts" />

	<PageTabs
		tabs={TABS}
		{activeTab}
		onTabChange={setTab}
		actionLabel={newAction?.label}
		onAction={newAction?.onclick}
	/>

	<InfoCallout>{tabDescriptions[activeTab]}</InfoCallout>

	{#if activeTab === 'channels'}
		{#if channelsLoading}
			<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if channelsError}
			<ErrorRetryBox message={channelsError} onRetry={() => loadChannels()} />
		{:else if channels.length === 0}
			<EmptyState
				message="No channels yet. Create one to get started."
				actionLabel="Create your first Channel"
				onAction={openNewChannel}
			/>
		{:else}
			<TableContainer>
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
						{#each channels as channel, __index (__index)}
							<Table.Row>
								<Table.Cell class="font-medium">{channel.name}</Table.Cell>
								<Table.Cell>
									<Badge variant="outline"
										>{channelTypeLabels[channel.channelType] || channel.channelType}</Badge
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
			</TableContainer>
		{/if}
	{:else if activeTab === 'rules'}
		{#if rulesLoading}
			<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
		{:else if rulesError}
			<ErrorRetryBox message={rulesError} onRetry={() => loadRules()} />
		{:else if rules.length === 0}
			<EmptyState
				message="No rules yet. Create one to get started."
				actionLabel="Create your first Rule"
				onAction={openNewRule}
			/>
		{:else}
			<TableContainer>
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
						{#each rules as rule, __index (__index)}
							<Table.Row>
								<Table.Cell class="font-medium">{rule.name}</Table.Cell>
								<Table.Cell>{ruleTypeLabels[rule.ruleType] || rule.ruleType}</Table.Cell>
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
			</TableContainer>
		{/if}
	{:else if activeTab === 'history'}
		<div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
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

		{#if historyError && !historyLoading}
			<ErrorRetryBox message={historyError} onRetry={() => loadHistory(false)} />
		{:else}
			<TableContainer>
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
							{#each history as item, __index (__index)}
								<Table.Row>
									<Table.Cell>
										<PageBadges severity={item.severity} fallback />
									</Table.Cell>
									<Table.Cell class="font-medium">{item.ruleName}</Table.Cell>
									<Table.Cell class="max-w-xs truncate">
										{#if item.url}
											<a
												{...{ href: resolveHref(item.url) }}
												class="text-blue-600 hover:underline dark:text-blue-400">{item.subject}</a
											>
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
										{:else if item.status === 'deduped'}
											<Badge variant="secondary" title="Folded into a page that was already open">
												Deduped
											</Badge>
										{:else}
											<Badge variant="secondary">Skipped</Badge>
										{/if}
									</Table.Cell>
									<Table.Cell class="text-muted-foreground">{formatDate(item.createdAt)}</Table.Cell
									>
								</Table.Row>
							{/each}
						</Table.Body>
					{/if}
				</Table.Root>
			</TableContainer>
		{/if}

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

<ConfirmDeleteDialog
	bind:open={showDeleteChannelDialog}
	entity="Channel"
	description={`Are you sure you want to delete "${deletingChannel?.name}"? This action cannot be undone.`}
	loading={deleteChannelLoading}
	onConfirm={deleteChannel}
/>

<ConfirmDeleteDialog
	bind:open={showDeleteRuleDialog}
	entity="Rule"
	description={`Are you sure you want to delete "${deletingRule?.name}"? This action cannot be undone.`}
	loading={deleteRuleLoading}
	onConfirm={deleteRule}
/>

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
			<Button
				variant="outline"
				onclick={() => (showToggleRuleDialog = false)}
				disabled={togglingLoading}>Cancel</Button
			>
			<Button onclick={confirmToggleRule} disabled={togglingLoading}>
				{#if togglingRule?.enabled}
					<ZapOff class="mr-2 h-4 w-4" />
					Disable Rule
				{:else}
					<Zap class="mr-2 h-4 w-4" />
					Enable Rule
				{/if}
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
				{#if testingChannel?.channelType === 'escalation'}
					Testing "{testingChannel?.name}" opens a real page and notifies whoever is on call,
					exactly as a firing rule would. Resolve the page from On-Call when you are done.
				{:else}
					Send a test notification to "{testingChannel?.name}"? This will deliver a test message
					through the configured channel.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={testError} />
		<AlertDialog.Footer>
			<Button
				variant="outline"
				onclick={() => (showTestChannelDialog = false)}
				disabled={testingLoading}>Cancel</Button
			>
			<Button onclick={confirmTestChannel} disabled={testingLoading}>
				<Send class="h-4 w-4" />
				{testingLoading ? 'Sending...' : 'Send Test'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
