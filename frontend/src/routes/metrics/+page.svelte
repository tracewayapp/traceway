<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import WidgetGrid from '$lib/components/dashboard/widget-grid.svelte';
	import WidgetConfigPanel from '$lib/components/dashboard/widget-config-panel.svelte';
	import { api } from '$lib/api';
	import { projectsState, FRAMEWORK_LABELS, type Framework } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { toUTCISO, calendarDateTimeToLuxon, formatDateTime } from '$lib/utils/formatters';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import {
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		updateUrl
	} from '$lib/utils/url-params';
	import { CalendarDate } from '@internationalized/date';
	import { Trash2, Plus, Pencil, Check, RefreshCw, CircleAlert, EllipsisVertical, Sparkles } from 'lucide-svelte';
	import * as Alert from '$lib/components/ui/alert';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { toast } from 'svelte-sonner';
	import type { DiscoveredMetric } from '$lib/types/dashboard';

	const timezone = $derived(getTimezone());

	type Widget = {
		id: number;
		title: string;
		widgetType: string;
		config: any;
		position: number;
		isStarred?: boolean;
	};

	type WidgetGroup = {
		id: number;
		name: string;
		description: string;
		isDefault: boolean;
		createdAt: string;
	};

	type WidgetGroupWithWidgets = WidgetGroup & {
		widgets: Widget[];
	};

	let widgetGroups = $state<WidgetGroup[]>([]);
	let projectFramework = $state<Framework | ''>('');
	let canPopulateDefaults = $state(false);
	let populating = $state(false);
	let loading = $state(true);
	let activeTabId = $state<string>('');
	let activeGroup = $state<WidgetGroupWithWidgets | null>(null);
	let loadingGroup = $state(false);

	let showCreateDialog = $state(false);
	let newName = $state('');
	let creating = $state(false);
	let createError = $state('');

	let showDeleteDialog = $state(false);
	let deleting = $state(false);
	let deleteError = $state('');

	let showRenameDialog = $state(false);
	let renameName = $state('');
	let renaming = $state(false);
	let renameError = $state('');

	let showWidgetConfig = $state(false);
	let editingWidget = $state<Widget | null>(null);
	let availableMetrics = $state<DiscoveredMetric[]>([]);
	let widgetConfigError = $state('');

	let showDeleteWidgetDialog = $state(false);
	let deletingWidget = $state<Widget | null>(null);
	let deletingWidgetLoading = $state(false);
	let deleteWidgetError = $state('');

	let lastUpdated = $state<Date | null>(null);

	let tabsListEl = $state<HTMLElement | null>(null);
	let hasOverflow = $state(false);

	const initialUrlParams = parseTimeRangeFromUrl(timezone);
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset ?? '24h');
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));
	let sharedTimeDomain = $state<[Date, Date] | null>(null);

	const lastUpdatedFormatted = $derived(
		lastUpdated ? formatDateTime(lastUpdated, { timezone, format: 'time' }) : ''
	);

	function reloadGroupWidgets() {
		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
		if (activeTabId) loadGroupWidgets(activeTabId);
	}

	function checkOverflow() {
		if (tabsListEl) {
			hasOverflow = tabsListEl.scrollWidth > tabsListEl.clientWidth;
		}
	}

	$effect(() => {
		if (!tabsListEl) return;
		const observer = new ResizeObserver(() => checkOverflow());
		observer.observe(tabsListEl);
		checkOverflow();
		return () => observer.disconnect();
	});

	$effect(() => {
		widgetGroups;
		if (tabsListEl) {
			requestAnimationFrame(() => checkOverflow());
		}
	});

	function getFromDateTimeUTC(): string {
		const [hour, minute] = (fromTime || '00:00').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: fromDate.year, month: fromDate.month, day: fromDate.day, hour, minute },
			timezone
		);
		return toUTCISO(dt);
	}

	function getToDateTimeUTC(): string {
		const [hour, minute] = (toTime || '23:59').split(':').map(Number);
		const dt = calendarDateTimeToLuxon(
			{ year: toDate.year, month: toDate.month, day: toDate.day, hour, minute },
			timezone
		).endOf('minute');
		return toUTCISO(dt);
	}

	async function loadWidgetGroups() {
		loading = true;
		try {
			const response = await api.get('/widget-groups', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			widgetGroups = response.widgetGroups || [];
			projectFramework = (response.framework as Framework) || '';
			canPopulateDefaults = !!response.canPopulateDefaults;
			if (widgetGroups.length > 0 && !activeTabId) {
				activeTabId = String(widgetGroups[0].id);
			}
		} catch (e) {
			console.error('Failed to load widget groups:', e);
		} finally {
			loading = false;
		}
	}

	async function populateDefaults() {
		populating = true;
		try {
			await api.post(
				'/widget-groups/populate-defaults',
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully populated default charts', { position: 'top-center' });
			await loadWidgetGroups();
		} catch (e: any) {
			toast.error(e?.message || 'Failed to populate default charts', { position: 'top-center' });
		} finally {
			populating = false;
		}
	}

	const frameworkLabel = $derived(
		projectFramework ? (FRAMEWORK_LABELS[projectFramework] ?? projectFramework) : ''
	);

	async function loadGroupWidgets(groupId: string) {
		loadingGroup = true;
		try {
			const result = await api.get(`/widget-groups/${groupId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			activeGroup = result;
			lastUpdated = new Date();
		} catch (e) {
			console.error('Failed to load widget group:', e);
			activeGroup = null;
		} finally {
			loadingGroup = false;
		}
	}

	$effect(() => {
		if (activeTabId) {
			loadGroupWidgets(activeTabId);
		}
	});

	$effect(() => {
		if (!showWidgetConfig) {
			widgetConfigError = '';
		}
	});

	function handleTabChange(tab: string) {
		activeTabId = tab;
		showWidgetConfig = false;
		editingWidget = null;
	}

	async function createWidgetGroup() {
		creating = true;
		createError = '';
		try {
			const group = await api.post(
				'/widget-groups',
				{ name: newName },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully created a new Widget Group', { position: 'top-center' });
			showCreateDialog = false;
			newName = '';
			await loadWidgetGroups();
			activeTabId = String(group.id);
		} catch (e: any) {
			if (e?.status === 422) {
				createError = e.message;
			} else {
				console.error('Failed to create widget group:', e);
			}
		} finally {
			creating = false;
		}
	}

	function openRenameDialog() {
		renameName = activeGroup?.name ?? activeTabName;
		renameError = '';
		showRenameDialog = true;
	}

	async function renameWidgetGroup() {
		if (!activeGroup) return;
		renaming = true;
		renameError = '';
		try {
			await api.put(
				`/widget-groups/${activeGroup.id}`,
				{ name: renameName, description: activeGroup.description },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully updated the Widget Group', { position: 'top-center' });
			showRenameDialog = false;
			activeGroup.name = renameName;
			await loadWidgetGroups();
		} catch (e: any) {
			if (e?.status === 422) {
				renameError = e.message;
			} else {
				console.error('Failed to rename widget group:', e);
			}
		} finally {
			renaming = false;
		}
	}

	async function deleteWidgetGroup() {
		if (!activeGroup) return;
		deleting = true;
		deleteError = '';
		try {
			await api.delete(`/widget-groups/${activeGroup.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			showDeleteDialog = false;
			activeTabId = '';
			activeGroup = null;
			await loadWidgetGroups();
			if (widgetGroups.length > 0) {
				activeTabId = String(widgetGroups[0].id);
			}
		} catch (e: any) {
			if (e?.status === 422) {
				deleteError = e.message;
			} else {
				console.error('Failed to delete widget group:', e);
			}
		} finally {
			deleting = false;
		}
	}

	function handleTimeRangeChange(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		fromDate = from.date;
		fromTime = from.time;
		toDate = to.date;
		toTime = to.time;
		selectedPreset = preset;

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
		updateUrl({ preset: selectedPreset ?? undefined });
	}

	function handleChartRangeSelect(from: Date, to: Date) {
		selectedPreset = null;
		fromDate = new CalendarDate(from.getFullYear(), from.getMonth() + 1, from.getDate());
		fromTime = `${String(from.getHours()).padStart(2, '0')}:${String(from.getMinutes()).padStart(2, '0')}`;
		toDate = new CalendarDate(to.getFullYear(), to.getMonth() + 1, to.getDate());
		toTime = `${String(to.getHours()).padStart(2, '0')}:${String(to.getMinutes()).padStart(2, '0')}`;
		sharedTimeDomain = [from, to];
	}

	async function loadMetrics() {
		try {
			const discoverResponse = await api.get(
				`/metrics/discover?from=${getFromDateTimeUTC()}&to=${getToDateTimeUTC()}`,
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			availableMetrics = discoverResponse.metrics || [];
		} catch {
			// ignore
		}
	}

	function openAddWidget() {
		editingWidget = null;
		showWidgetConfig = true;
		loadMetrics();
	}

	function openEditWidget(widget: Widget) {
		editingWidget = widget;
		showWidgetConfig = true;
		loadMetrics();
	}

	function openDeleteWidgetDialog(widget: Widget) {
		deletingWidget = widget;
		showDeleteWidgetDialog = true;
	}

	async function confirmDeleteWidget() {
		if (!activeGroup || !deletingWidget?.id) return;
		deletingWidgetLoading = true;
		deleteWidgetError = '';
		try {
			await api.delete(`/widget-groups/${activeGroup.id}/widgets/${deletingWidget.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			showDeleteWidgetDialog = false;
			deletingWidget = null;
			await loadGroupWidgets(activeTabId);
		} catch (e: any) {
			if (e?.status === 422) {
				deleteWidgetError = e.message;
			} else {
				console.error('Failed to delete widget:', e);
			}
		} finally {
			deletingWidgetLoading = false;
		}
	}

	async function handleToggleStar(widget: { id: number; isStarred?: boolean }) {
		if (!activeGroup) return;
		const target = activeGroup.widgets?.find((w) => w.id === widget.id);
		if (!target) return;
		const previous = target.isStarred ?? false;
		target.isStarred = !previous;
		try {
			await api.put(
				`/widget-groups/${activeGroup.id}/widgets/${widget.id}/star`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e: any) {
			target.isStarred = previous;
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to update star', { position: 'top-center' });
			}
		}
	}

	async function handleReorderWidgets(widgetIds: number[]) {
		if (!activeGroup) return;
		const previousPositions = new Map(activeGroup.widgets.map((w) => [w.id, w.position]));
		for (const w of activeGroup.widgets) {
			w.position = widgetIds.indexOf(w.id);
		}
		try {
			await api.put(
				`/widget-groups/${activeGroup.id}/reorder`,
				{ widgetIds },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e: any) {
			for (const w of activeGroup.widgets) {
				w.position = previousPositions.get(w.id) ?? w.position;
			}
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to reorder widgets', { position: 'top-center' });
			}
			await loadGroupWidgets(activeTabId);
		}
	}

	async function handleDuplicateWidget(widget: Widget) {
		if (!activeGroup) return;
		try {
			await api.post(
				`/widget-groups/${activeGroup.id}/widgets`,
				{
					title: `${widget.title} (copy)`,
					widgetType: widget.widgetType,
					config: widget.config
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully duplicated the Widget', { position: 'top-center' });
			await loadGroupWidgets(activeTabId);
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to duplicate widget', { position: 'top-center' });
			}
		}
	}

	async function handleResizeWidget(widget: Widget, layout: { colSpan: number; size: string }) {
		if (!activeGroup) return;
		const target = activeGroup.widgets?.find((w) => w.id === widget.id);
		if (!target) return;
		const previousConfig = target.config;
		target.config = { ...(target.config ?? {}), colSpan: layout.colSpan, size: layout.size };
		try {
			await api.put(
				`/widget-groups/${activeGroup.id}/widgets/${widget.id}`,
				{ title: target.title, widgetType: target.widgetType, config: target.config },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e: any) {
			target.config = previousConfig;
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to resize widget', { position: 'top-center' });
			}
		}
	}

	async function handleWidgetSave(data: { title: string; widgetType: string; config: any }) {
		if (!activeGroup) return;

		widgetConfigError = '';
		try {
			if (editingWidget?.id) {
				await api.put(`/widget-groups/${activeGroup.id}/widgets/${editingWidget.id}`, data, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Widget', { position: 'top-center' });
			} else {
				await api.post(`/widget-groups/${activeGroup.id}/widgets`, data, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully added the Widget', { position: 'top-center' });
			}
			showWidgetConfig = false;
			editingWidget = null;
			await loadGroupWidgets(activeTabId);
		} catch (e: any) {
			if (e?.status === 422) {
				widgetConfigError = e.message;
			} else {
				console.error('Failed to save widget:', e);
			}
		}
	}

	const activeTabName = $derived(
		widgetGroups.find((g) => String(g.id) === activeTabId)?.name ?? ''
	);

	onMount(() => {
		loadWidgetGroups();

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
	});
</script>

<div class="space-y-4">
	<div class="mb-2 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h2 class="text-2xl font-bold tracking-tight">Metrics</h2>
		<div class="flex items-center gap-2">
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

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if widgetGroups.length === 0}
		<div
			class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center"
		>
			<p class="mb-4 text-muted-foreground">No widget groups yet.</p>
			<div class="flex flex-wrap items-center justify-center gap-2">
				{#if canPopulateDefaults}
					<Button onclick={populateDefaults} disabled={populating}>
						<Sparkles class="mr-1 h-4 w-4" />
						{populating
							? 'Populating...'
							: frameworkLabel
								? `Populate default charts for ${frameworkLabel}`
								: 'Populate default charts'}
					</Button>
				{/if}
				<Button variant="outline" onclick={() => (showCreateDialog = true)}>
					<Plus class="mr-1 h-4 w-4" />
					Create your own Widget Group
				</Button>
			</div>
		</div>
	{:else}
		<Tabs.Root value={activeTabId} onValueChange={handleTabChange}>
			<div class="flex items-center gap-2">
				{#if hasOverflow}
					<Select.Root
						type="single"
						value={activeTabId}
						onValueChange={(v) => {
							if (v) handleTabChange(v);
						}}
					>
						<Select.Trigger size="sm">{activeTabName}</Select.Trigger>
						<Select.Content>
							{#each widgetGroups as group (group.id)}
								<Select.Item value={String(group.id)}>{group.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/if}
				<div
					class="flex min-w-0 items-center overflow-hidden"
					class:invisible={hasOverflow}
					class:h-0={hasOverflow}
					class:w-0={hasOverflow}
					bind:this={tabsListEl}
				>
					<Tabs.List>
						{#each widgetGroups as group (group.id)}
							<Tabs.Trigger value={String(group.id)}>
								{group.name}
								{#if String(group.id) === activeTabId}
									<DropdownMenu.Root>
										<DropdownMenu.Trigger>
											{#snippet child({ props })}
												<span
													{...props}
													class="-mr-1 ml-1 inline-flex h-5 w-5 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
													role="button"
													tabindex={0}
													onclick={(e) => e.stopPropagation()}
													onkeydown={(e) => {
														if (e.key === 'Enter' || e.key === ' ') e.stopPropagation();
													}}
												>
													<EllipsisVertical class="h-3.5 w-3.5" />
												</span>
											{/snippet}
										</DropdownMenu.Trigger>
										<DropdownMenu.Content align="start">
											<DropdownMenu.Item onclick={openRenameDialog}>
												<Pencil class="mr-2 h-4 w-4" />
												Rename Group
											</DropdownMenu.Item>
											<DropdownMenu.Separator />
											<DropdownMenu.Item
												class="text-destructive"
												onclick={() => (showDeleteDialog = true)}
											>
												<Trash2 class="mr-2 h-4 w-4" />
												Delete Group
											</DropdownMenu.Item>
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								{/if}
							</Tabs.Trigger>
						{/each}
					</Tabs.List>
				</div>
				<button
					class="inline-flex items-center justify-center rounded-md px-2 py-1 text-muted-foreground transition-all hover:bg-muted hover:text-foreground"
					onclick={() => (showCreateDialog = true)}
				>
					<Plus class="h-4 w-4" />
				</button>
				{#if hasOverflow && activeTabId}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
						>
							<EllipsisVertical class="h-4 w-4" />
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end">
							<DropdownMenu.Item onclick={openRenameDialog}>
								<Pencil class="mr-2 h-4 w-4" />
								Rename Group
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
							<DropdownMenu.Item class="text-destructive" onclick={() => (showDeleteDialog = true)}>
								<Trash2 class="mr-2 h-4 w-4" />
								Delete Group
							</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
				{#if lastUpdated}
					<div class="ml-auto flex items-center gap-1">
						<span class="text-sm whitespace-nowrap text-muted-foreground">
							Updated: {lastUpdatedFormatted}
						</span>
						<Button variant="ghost" size="sm" onclick={reloadGroupWidgets} disabled={loadingGroup}>
							<RefreshCw class="h-4 w-4 {loadingGroup ? 'animate-spin' : ''}" />
						</Button>
					</div>
				{/if}
			</div>

			{#each widgetGroups as group (group.id)}
				<Tabs.Content value={String(group.id)}>
					{#if loadingGroup && activeTabId === String(group.id)}
						<div class="flex items-center justify-center py-20">
							<LoadingCircle size="xlg" />
						</div>
					{:else if activeGroup && activeTabId === String(group.id)}
						<WidgetGrid
							widgets={activeGroup.widgets ?? []}
							fromDateUTC={getFromDateTimeUTC()}
							toDateUTC={getToDateTimeUTC()}
							timeDomain={sharedTimeDomain}
							onEditWidget={openEditWidget}
							onDeleteWidget={openDeleteWidgetDialog}
							onReorderWidgets={handleReorderWidgets}
							onDuplicateWidget={handleDuplicateWidget}
							onResizeWidget={handleResizeWidget}
							onAddWidget={openAddWidget}
							onToggleStar={handleToggleStar}
							onRangeSelect={handleChartRangeSelect}
						/>
					{/if}
				</Tabs.Content>
			{/each}
		</Tabs.Root>
	{/if}
</div>

<WidgetConfigPanel
	bind:open={showWidgetConfig}
	widget={editingWidget}
	{availableMetrics}
	error={widgetConfigError}
	onSave={handleWidgetSave}
	onCancel={() => {
		showWidgetConfig = false;
		editingWidget = null;
		widgetConfigError = '';
	}}
/>

<AlertDialog.Root
	bind:open={showCreateDialog}
	onOpenChange={(o) => {
		if (!o) createError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>New Widget Group</AlertDialog.Title>
		</AlertDialog.Header>
		{#if createError}
			<Alert.Root variant="destructive" class="border-red-200 bg-red-50">
				<CircleAlert class="h-4 w-4 text-red-700" />
				<Alert.Title class="text-red-800">Error</Alert.Title>
				<Alert.Description class="text-red-700">{createError}</Alert.Description>
			</Alert.Root>
		{/if}
		<div class="space-y-4">
			<div>
				<label class="text-sm font-medium" for="new-group-name">Name</label>
				<Input id="new-group-name" bind:value={newName} placeholder="My Group" maxlength={12} />
			</div>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showCreateDialog = false)}>Cancel</Button>
			<Button onclick={createWidgetGroup} disabled={creating}>
				{#if creating}Creating...{:else}<Plus class="mr-1 h-4 w-4" /> New Widget Group{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showRenameDialog}
	onOpenChange={(o) => {
		if (!o) renameError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Rename Widget Group</AlertDialog.Title>
		</AlertDialog.Header>
		{#if renameError}
			<Alert.Root variant="destructive" class="border-red-200 bg-red-50">
				<CircleAlert class="h-4 w-4 text-red-700" />
				<Alert.Title class="text-red-800">Error</Alert.Title>
				<Alert.Description class="text-red-700">{renameError}</Alert.Description>
			</Alert.Root>
		{/if}
		<div class="space-y-4">
			<div>
				<label class="text-sm font-medium" for="rename-group-name">Name</label>
				<Input
					id="rename-group-name"
					bind:value={renameName}
					placeholder="My Group"
					maxlength={12}
				/>
			</div>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showRenameDialog = false)}>Cancel</Button>
			<Button onclick={renameWidgetGroup} disabled={renaming}>
				{#if renaming}Updating...{:else}<Check class="mr-1 h-4 w-4" /> Update Widget Group{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showDeleteDialog}
	onOpenChange={(o) => {
		if (!o) deleteError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Widget Group</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{activeGroup?.name}"? This will remove all widgets in this
				group. This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if deleteError}
			<Alert.Root variant="destructive" class="border-red-200 bg-red-50">
				<CircleAlert class="h-4 w-4 text-red-700" />
				<Alert.Title class="text-red-800">Error</Alert.Title>
				<Alert.Description class="text-red-700">{deleteError}</Alert.Description>
			</Alert.Root>
		{/if}
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showDeleteDialog = false)} disabled={deleting}
				>Cancel</Button
			>
			<Button variant="destructive" onclick={deleteWidgetGroup} disabled={deleting}>
				{deleting ? 'Deleting...' : 'Delete'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showDeleteWidgetDialog}
	onOpenChange={(o) => {
		if (!o) deleteWidgetError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Widget</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{deletingWidget?.title}"? This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		{#if deleteWidgetError}
			<Alert.Root variant="destructive" class="border-red-200 bg-red-50">
				<CircleAlert class="h-4 w-4 text-red-700" />
				<Alert.Title class="text-red-800">Error</Alert.Title>
				<Alert.Description class="text-red-700">{deleteWidgetError}</Alert.Description>
			</Alert.Root>
		{/if}
		<AlertDialog.Footer>
			<Button
				variant="outline"
				onclick={() => (showDeleteWidgetDialog = false)}
				disabled={deletingWidgetLoading}>Cancel</Button
			>
			<Button variant="destructive" onclick={confirmDeleteWidget} disabled={deletingWidgetLoading}>
				{deletingWidgetLoading ? 'Deleting...' : 'Delete'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
