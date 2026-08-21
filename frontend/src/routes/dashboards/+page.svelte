<script lang="ts">
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import WidgetGrid from '$lib/components/dashboard/widget-grid.svelte';
	import WidgetConfigPanel from '$lib/components/dashboard/widget-config-panel.svelte';
	import DashboardCommand from '$lib/components/dashboard/dashboard-command.svelte';
	import { afterNavigate, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
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
	import {
		Trash2,
		Plus,
		Pencil,
		Check,
		RefreshCw,
		EllipsisVertical,
		Braces,
		Download,
		Upload,
		Share2,
		CircleMinus,
		Search,
		Server,
		X
	} from '@lucide/svelte';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { ErrorDisplay } from '$lib/components/ui/error-display';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import FormField from '$lib/components/traceway/form-field.svelte';
	import EmptyState from '$lib/components/traceway/empty-state.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { toast } from 'svelte-sonner';
	import type { DashboardWidgetConfig, DiscoveredMetric } from '$lib/types/dashboard';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	type Widget = {
		id: string;
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
		position: number;
		isStarred?: boolean;
	};

	type Dashboard = {
		id: number;
		name: string;
		description: string;
		organizationId: number;
		templateKey: string | null;
	};

	type DashboardWithWidgets = Dashboard & {
		appliedProjectIds: string[];
		widgets: Widget[];
	};

	let dashboards = $state<Dashboard[]>([]);
	let loading = $state(true);
	let listError = $state('');
	let activeTabId = $state<string>('');
	let activeDashboard = $state<DashboardWithWidgets | null>(null);
	let loadingDashboard = $state(false);
	let loadError = $state('');

	let showCreateDialog = $state(false);
	let newName = $state('');
	let creating = $state(false);
	let createError = $state('');

	let showDeleteDialog = $state(false);
	let deleting = $state(false);
	let deleteError = $state('');

	let showRemoveDialog = $state(false);
	let removing = $state(false);
	let removeError = $state('');

	let showRenameDialog = $state(false);
	let renameName = $state('');
	let renaming = $state(false);
	let renameError = $state('');

	let showJsonDialog = $state(false);
	let jsonText = $state('');
	let jsonSaving = $state(false);
	let jsonError = $state('');

	let showApplyDialog = $state(false);
	let applySelection = $state<Record<string, boolean>>({});
	let applying = $state(false);
	let applyError = $state('');

	let showImportDialog = $state(false);
	let importText = $state('');
	let importing = $state(false);
	let importError = $state('');

	let showGrafanaDialog = $state(false);
	let grafanaText = $state('');
	let grafanaImporting = $state(false);
	let grafanaError = $state('');
	let grafanaWarnings = $state<string[]>([]);

	let showWidgetConfig = $state(false);
	let editingWidget = $state<Widget | null>(null);
	let widgetPrefill = $state<{
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
	} | null>(null);
	let availableMetrics = $state<DiscoveredMetric[]>([]);
	let widgetConfigError = $state('');

	let showPalette = $state(false);
	const isMac = typeof navigator !== 'undefined' && /Mac|iP(hone|ad|od)/.test(navigator.platform);

	function handlePageKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			if (isMac && !e.metaKey) return;
			e.preventDefault();
			showPalette = !showPalette;
		}
	}

	let showDeleteWidgetDialog = $state(false);
	let deletingWidget = $state<Widget | null>(null);
	let deletingWidgetLoading = $state(false);
	let deleteWidgetError = $state('');

	let lastUpdated = $state<Date | null>(null);

	let tabsListEl = $state<HTMLElement | null>(null);
	let tabsRowEl = $state<HTMLElement | null>(null);
	let plusBtnEl = $state<HTMLElement | null>(null);
	let updatedEl = $state<HTMLElement | null>(null);
	let hasOverflow = $state(false);

	const initialUrlParams = parseTimeRangeFromUrl(initialTimezone);
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset ?? '24h');
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));
	let sharedTimeDomain = $state<[Date, Date] | null>(null);

	const lastUpdatedFormatted = $derived(
		lastUpdated ? formatDateTime(lastUpdated, { timezone, format: 'time' }) : ''
	);

	const orgProjects = $derived(
		activeDashboard
			? projectsState.projects.filter((p) => p.organizationId === activeDashboard?.organizationId)
			: []
	);
	let serverScope = $state(page.url.searchParams.get('server')?.trim() ?? '');
	const scopeTagFilters = $derived.by(
		(): Record<string, string> => (serverScope ? { server_name: serverScope } : {})
	);

	function clearServerScope() {
		const url = new URL(window.location.href);
		url.searchParams.delete('server');
		serverScope = '';
		replaceState(url.pathname + url.search, {});
	}

	function updateDashboardUrl(params: Record<string, string>) {
		updateUrl(serverScope ? { ...params, server: serverScope } : params);
	}

	function reloadDashboardWidgets() {
		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
		if (activeTabId) loadDashboardWidgets(activeTabId);
	}

	function checkOverflow() {
		if (!tabsRowEl || !tabsListEl) return;
		const reserved = (plusBtnEl?.offsetWidth ?? 0) + (updatedEl?.offsetWidth ?? 0) + 24;
		hasOverflow = tabsListEl.scrollWidth > tabsRowEl.clientWidth - reserved;
	}

	$effect(() => {
		if (!tabsRowEl) return;
		const observer = new ResizeObserver(() => checkOverflow());
		observer.observe(tabsRowEl);
		checkOverflow();
		return () => observer.disconnect();
	});

	$effect(() => {
		void dashboards;
		void lastUpdated;
		if (tabsRowEl) {
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

	async function loadDashboards() {
		loading = true;
		listError = '';
		try {
			const response = await api.get('/dashboards', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			dashboards = response.dashboards || [];
			if (dashboards.length > 0 && !dashboards.some((d) => String(d.id) === activeTabId)) {
				activeTabId = String(dashboards[0].id);
			}
		} catch (e) {
			console.error('Failed to load dashboards:', e);
			listError = getErrorMessage(e) || 'Failed to load dashboards';
		} finally {
			loading = false;
		}
	}

	let loadSeq = 0;

	async function loadDashboardWidgets(dashboardId: string) {
		const seq = ++loadSeq;
		loadingDashboard = true;
		loadError = '';
		try {
			const result = await api.get(`/dashboards/${dashboardId}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			if (seq !== loadSeq) return;
			activeDashboard = {
				...result,
				widgets: (result.widgets ?? []).map((w: Widget, i: number) => ({ ...w, position: i }))
			};
			lastUpdated = new Date();
		} catch (e) {
			if (seq !== loadSeq) return;
			console.error('Failed to load dashboard:', e);
			activeDashboard = null;
			loadError = 'Failed to load the dashboard.';
		} finally {
			if (seq === loadSeq) {
				loadingDashboard = false;
			}
		}
	}

	$effect(() => {
		if (activeTabId) {
			loadDashboardWidgets(activeTabId);
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

	let tabDragIndex = $state<number | null>(null);
	let tabDropIndex = $state<number | null>(null);

	function handleTabDragStart(e: DragEvent, i: number) {
		tabDragIndex = i;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', String(dashboards[i].id));
		}
	}

	function handleTabDragOver(e: DragEvent, i: number) {
		if (tabDragIndex === null) return;
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		tabDropIndex = i;
	}

	function handleTabDrop(e: DragEvent, targetIndex: number) {
		e.preventDefault();
		if (tabDragIndex !== null && tabDragIndex !== targetIndex) {
			const order = dashboards.map((d) => d.id);
			const [moved] = order.splice(tabDragIndex, 1);
			order.splice(targetIndex, 0, moved);
			reorderDashboards(order);
		}
		tabDragIndex = null;
		tabDropIndex = null;
	}

	function handleTabDragEnd() {
		tabDragIndex = null;
		tabDropIndex = null;
	}

	async function reorderDashboards(dashboardIds: number[]) {
		const previous = dashboards;
		const byId = new Map(dashboards.map((d) => [d.id, d]));
		dashboards = dashboardIds
			.map((id) => byId.get(id))
			.filter((d): d is Dashboard => d !== undefined);
		try {
			await api.put(
				'/dashboards/reorder',
				{ dashboardIds },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			dashboards = previous;
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to reorder dashboards');
			}
			await loadDashboards();
		}
	}

	async function createDashboard() {
		creating = true;
		createError = '';
		try {
			const dashboard = await api.post(
				'/dashboards',
				{ name: newName },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully created the Dashboard', { position: 'top-center' });
			showCreateDialog = false;
			newName = '';
			await loadDashboards();
			activeTabId = String(dashboard.id);
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				createError = getErrorMessage(e);
			} else {
				console.error('Failed to create dashboard:', e);
			}
		} finally {
			creating = false;
		}
	}

	function openRenameDialog() {
		renameName = activeDashboard?.name ?? activeTabName;
		renameError = '';
		showRenameDialog = true;
	}

	async function renameDashboard() {
		if (!activeDashboard) return;
		renaming = true;
		renameError = '';
		try {
			await api.put(
				`/dashboards/${activeDashboard.id}`,
				{ name: renameName },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully updated the Dashboard', { position: 'top-center' });
			showRenameDialog = false;
			activeDashboard.name = renameName;
			await loadDashboards();
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				renameError = getErrorMessage(e);
			} else {
				console.error('Failed to rename dashboard:', e);
			}
		} finally {
			renaming = false;
		}
	}

	async function deleteDashboard() {
		if (!activeDashboard) return;
		deleting = true;
		deleteError = '';
		try {
			await api.delete(`/dashboards/${activeDashboard.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			showDeleteDialog = false;
			activeTabId = '';
			activeDashboard = null;
			await loadDashboards();
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				deleteError = getErrorMessage(e);
			} else {
				console.error('Failed to delete dashboard:', e);
			}
		} finally {
			deleting = false;
		}
	}

	async function removeFromProject() {
		if (!activeDashboard || !projectsState.currentProjectId) return;
		removing = true;
		removeError = '';
		try {
			await api.delete(
				`/dashboards/${activeDashboard.id}/apply/${projectsState.currentProjectId}`,
				{
					projectId: projectsState.currentProjectId ?? undefined
				}
			);
			showRemoveDialog = false;
			activeTabId = '';
			activeDashboard = null;
			await loadDashboards();
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				removeError = getErrorMessage(e);
			} else {
				console.error('Failed to remove dashboard from project:', e);
			}
		} finally {
			removing = false;
		}
	}

	function openJsonDialog() {
		if (!activeDashboard) return;
		jsonError = '';
		jsonText = JSON.stringify(
			{
				schemaVersion: 1,
				name: activeDashboard.name,
				description: activeDashboard.description,
				widgets: activeDashboard.widgets.map((w) => ({
					id: w.id,
					title: w.title,
					widgetType: w.widgetType,
					config: w.config
				}))
			},
			null,
			2
		);
		showJsonDialog = true;
	}

	async function saveJson() {
		if (!activeDashboard) return;
		jsonError = '';
		let parsed: unknown;
		try {
			parsed = JSON.parse(jsonText);
		} catch {
			jsonError = 'The document is not valid JSON.';
			return;
		}
		if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
			jsonError = 'The document must contain a JSON object.';
			return;
		}
		const document = parsed as Record<string, unknown>;
		jsonSaving = true;
		try {
			await api.put(
				`/dashboards/${activeDashboard.id}`,
				{
					name: document.name,
					description: document.description ?? '',
					definition: { schemaVersion: 1, widgets: document.widgets ?? [] }
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully updated the Dashboard', { position: 'top-center' });
			showJsonDialog = false;
			await loadDashboards();
			await loadDashboardWidgets(activeTabId);
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				jsonError = getErrorMessage(e);
			} else {
				console.error('Failed to save dashboard JSON:', e);
			}
		} finally {
			jsonSaving = false;
		}
	}

	function downloadJson(filename: string, data: unknown) {
		const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		a.click();
		URL.revokeObjectURL(url);
	}

	async function exportDashboard() {
		if (!activeDashboard) return;
		try {
			const doc = await api.get(`/dashboards/${activeDashboard.id}/export`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			const slug = activeDashboard.name.toLowerCase().replace(/[^a-z0-9]+/g, '-');
			downloadJson(`${slug || 'dashboard'}.dashboard.json`, doc);
		} catch (e) {
			toast.error(getErrorMessage(e) || 'Failed to export dashboard');
		}
	}

	function openApplyDialog() {
		if (!activeDashboard) return;
		applyError = '';
		const selection: Record<string, boolean> = {};
		for (const p of orgProjects) {
			selection[p.id] = activeDashboard.appliedProjectIds.includes(p.id);
		}
		applySelection = selection;
		showApplyDialog = true;
	}

	async function saveApply() {
		if (!activeDashboard) return;
		applying = true;
		applyError = '';
		try {
			const projectIds = Object.keys(applySelection).filter((id) => applySelection[id]);
			await api.put(
				`/dashboards/${activeDashboard.id}/apply`,
				{ projectIds },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully updated the Dashboard', { position: 'top-center' });
			showApplyDialog = false;
			await loadDashboards();
			if (activeTabId && dashboards.some((d) => String(d.id) === activeTabId)) {
				await loadDashboardWidgets(activeTabId);
			}
		} catch (e) {
			if (getErrorStatus(e) === 422) {
				applyError = getErrorMessage(e);
			} else {
				applyError = getErrorMessage(e) || 'Failed to apply dashboard';
			}
		} finally {
			applying = false;
		}
	}

	function readFileInto(setter: (text: string) => void) {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.json,application/json';
		input.onchange = () => {
			const file = input.files?.[0];
			if (!file) return;
			const reader = new FileReader();
			reader.onload = () => setter(String(reader.result ?? ''));
			reader.readAsText(file);
		};
		input.click();
	}

	async function importDashboards() {
		importError = '';
		let parsed: unknown;
		try {
			parsed = JSON.parse(importText);
		} catch {
			importError = 'The document is not valid JSON.';
			return;
		}
		importing = true;
		try {
			const result = await api.post(
				'/dashboards/import',
				{ data: parsed },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			const count = (result.dashboards ?? []).length;
			toast.success(`Successfully imported ${count} Dashboard${count === 1 ? '' : 's'}`);
			showImportDialog = false;
			importText = '';
			await loadDashboards();
			if (result.dashboards?.length) {
				activeTabId = String(result.dashboards[0].id);
			}
		} catch (e) {
			if (getErrorStatus(e) === 422) {
				importError = getErrorMessage(e);
			} else {
				importError = getErrorMessage(e) || 'Failed to import dashboards';
			}
		} finally {
			importing = false;
		}
	}

	async function importGrafana() {
		grafanaError = '';
		let parsed: unknown;
		try {
			parsed = JSON.parse(grafanaText);
		} catch {
			grafanaError = 'The document is not valid JSON.';
			return;
		}
		grafanaImporting = true;
		try {
			const result = await api.post(
				'/dashboards/import/grafana',
				{ grafana: parsed },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully imported the Dashboard');
			grafanaWarnings = result.warnings ?? [];
			grafanaText = '';
			await loadDashboards();
			if (result.dashboard?.id) {
				activeTabId = String(result.dashboard.id);
			}
			if (grafanaWarnings.length === 0) {
				showGrafanaDialog = false;
			}
		} catch (e) {
			if (getErrorStatus(e) === 422) {
				grafanaError = getErrorMessage(e);
			} else {
				grafanaError = getErrorMessage(e) || 'Failed to import the Grafana dashboard';
			}
		} finally {
			grafanaImporting = false;
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
		if (selectedPreset) {
			updateDashboardUrl({ preset: selectedPreset });
		} else {
			updateDashboardUrl({ from: getFromDateTimeUTC(), to: getToDateTimeUTC() });
		}
	}

	function handleChartRangeSelect(from: Date, to: Date) {
		selectedPreset = null;
		fromDate = new CalendarDate(from.getFullYear(), from.getMonth() + 1, from.getDate());
		fromTime = `${String(from.getHours()).padStart(2, '0')}:${String(from.getMinutes()).padStart(2, '0')}`;
		toDate = new CalendarDate(to.getFullYear(), to.getMonth() + 1, to.getDate());
		toTime = `${String(to.getHours()).padStart(2, '0')}:${String(to.getMinutes()).padStart(2, '0')}`;
		sharedTimeDomain = [from, to];
		updateDashboardUrl({ from: getFromDateTimeUTC(), to: getToDateTimeUTC() });
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
		widgetPrefill = null;
		showWidgetConfig = true;
		loadMetrics();
	}

	async function openAddWidgetWithMetric(metricName: string) {
		if (!activeDashboard) {
			if (!activeTabId) return;
			await loadDashboardWidgets(activeTabId);
			if (!activeDashboard) return;
		}
		editingWidget = null;
		widgetPrefill = {
			title: metricName,
			widgetType: 'line_chart',
			config: { sources: [{ type: 'metric', name: metricName, aggregation: 'avg' }] }
		};
		showWidgetConfig = true;
		loadMetrics();
	}

	function openEditWidget(widget: Widget) {
		editingWidget = widget;
		widgetPrefill = null;
		showWidgetConfig = true;
		loadMetrics();
	}

	async function handlePaletteApplied(dashboardId: number) {
		await loadDashboards();
		if (String(dashboardId) === activeTabId) {
			await loadDashboardWidgets(activeTabId);
		} else if (dashboards.some((d) => String(d.id) === String(dashboardId))) {
			activeTabId = String(dashboardId);
		}
	}

	function openDeleteWidgetDialog(widget: Widget) {
		deletingWidget = widget;
		showDeleteWidgetDialog = true;
	}

	async function confirmDeleteWidget() {
		if (!activeDashboard || !deletingWidget?.id) return;
		deletingWidgetLoading = true;
		deleteWidgetError = '';
		try {
			await api.delete(`/dashboards/${activeDashboard.id}/widgets/${deletingWidget.id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			showDeleteWidgetDialog = false;
			deletingWidget = null;
			await loadDashboardWidgets(activeTabId);
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				deleteWidgetError = getErrorMessage(e);
			} else {
				console.error('Failed to delete widget:', e);
			}
		} finally {
			deletingWidgetLoading = false;
		}
	}

	async function handleToggleStar(widget: { id: number | string; isStarred?: boolean }) {
		if (!activeDashboard) return;
		const target = activeDashboard.widgets?.find((w) => w.id === String(widget.id));
		if (!target) return;
		const previous = target.isStarred ?? false;
		target.isStarred = !previous;
		try {
			await api.put(
				`/dashboards/${activeDashboard.id}/widgets/${target.id}/star`,
				{},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			target.isStarred = previous;
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to update star');
			}
		}
	}

	async function handleReorderWidgets(reorderedIds: (number | string)[]) {
		if (!activeDashboard) return;
		const widgetIds = reorderedIds.map(String);
		const previousPositions = new Map(activeDashboard.widgets.map((w) => [w.id, w.position]));
		for (const w of activeDashboard.widgets) {
			w.position = widgetIds.indexOf(w.id);
		}
		try {
			await api.put(
				`/dashboards/${activeDashboard.id}/widgets/reorder`,
				{ widgetIds },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			for (const w of activeDashboard.widgets) {
				w.position = previousPositions.get(w.id) ?? w.position;
			}
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to reorder widgets');
			}
			await loadDashboardWidgets(activeTabId);
		}
	}

	async function handleDuplicateWidget(widget: Widget) {
		if (!activeDashboard) return;
		try {
			await api.post(
				`/dashboards/${activeDashboard.id}/widgets`,
				{
					title: `${widget.title} (copy)`,
					widgetType: widget.widgetType,
					config: widget.config
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			toast.success('Successfully duplicated the Widget');
			await loadDashboardWidgets(activeTabId);
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to duplicate widget');
			}
		}
	}

	async function handleResizeWidget(widget: Widget, layout: { colSpan: number; size: string }) {
		if (!activeDashboard) return;
		const target = activeDashboard.widgets?.find((w) => w.id === widget.id);
		if (!target) return;
		const previousConfig = target.config;
		target.config = { ...(target.config ?? {}), colSpan: layout.colSpan, size: layout.size };
		try {
			await api.put(
				`/dashboards/${activeDashboard.id}/widgets/${widget.id}`,
				{ title: target.title, widgetType: target.widgetType, config: target.config },
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
		} catch (e) {
			target.config = previousConfig;
			if (getErrorStatus(e) !== 403) {
				toast.error(getErrorMessage(e) || 'Failed to resize widget');
			}
		}
	}

	async function handleWidgetSave(data: {
		title: string;
		widgetType: string;
		config: DashboardWidgetConfig;
	}) {
		if (!activeDashboard) return;

		widgetConfigError = '';
		try {
			if (editingWidget?.id) {
				await api.put(`/dashboards/${activeDashboard.id}/widgets/${editingWidget.id}`, data, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Widget', { position: 'top-center' });
			} else {
				await api.post(`/dashboards/${activeDashboard.id}/widgets`, data, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully created the Widget', { position: 'top-center' });
			}
			showWidgetConfig = false;
			editingWidget = null;
			widgetPrefill = null;
			await loadDashboardWidgets(activeTabId);
		} catch (e) {
			if (getErrorStatus(e) === 422 || getErrorStatus(e) === 403) {
				widgetConfigError = getErrorMessage(e);
			} else {
				console.error('Failed to save widget:', e);
			}
		}
	}

	const activeTabName = $derived(dashboards.find((d) => String(d.id) === activeTabId)?.name ?? '');

	let handledSearch = '';
	let routerReady = $state(false);

	afterNavigate(() => {
		routerReady = true;
		serverScope = page.url.searchParams.get('server')?.trim() ?? '';
	});

	$effect(() => {
		if (!routerReady) return;
		const search = page.url.search;
		if (search === handledSearch) return;
		handledSearch = search;

		const params = new SvelteURLSearchParams(search);
		const action = params.get('action');
		const dashboardParam = params.get('dashboard');
		const addMetric = params.get('addMetric');
		if (!action && !dashboardParam && !addMetric) return;

		params.delete('action');
		params.delete('dashboard');
		params.delete('addMetric');
		const query = params.toString();
		let target = resolve(page.url.pathname as '/');
		target += query ? '?' + query : '';
		replaceState(target, {});

		if (action === 'create') showCreateDialog = true;
		else if (action === 'import') showImportDialog = true;
		else if (action === 'grafana') showGrafanaDialog = true;

		if (dashboardParam || addMetric) {
			loadDashboards().then(() => {
				if (dashboardParam && dashboards.some((d) => String(d.id) === dashboardParam)) {
					if (dashboardParam === activeTabId) {
						loadDashboardWidgets(activeTabId);
					} else {
						activeTabId = dashboardParam;
					}
				}
				if (addMetric && dashboards.length > 0) {
					openAddWidgetWithMetric(addMetric);
				}
			});
		}
	});

	onMount(() => {
		loadDashboards();

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
	});

	// The resolved timezone can flip from the browser zone to the organization
	// zone once projects finish loading. The wall-clock range state above was
	// materialized under the old zone; re-resolve the preset under the new one
	// or every widget query shifts by the offset difference.
	let lastResolvedTimezone = getTimezone();
	$effect(() => {
		const tz = timezone;
		if (tz === lastResolvedTimezone) return;
		lastResolvedTimezone = tz;
		if (!selectedPreset) return;
		const range = getTimeRangeFromPreset(selectedPreset, tz);
		fromDate = dateToCalendarDate(range.from, tz);
		toDate = dateToCalendarDate(range.to, tz);
		fromTime = dateToTimeString(range.from, tz);
		toTime = dateToTimeString(range.to, tz);
		sharedTimeDomain = [new Date(getFromDateTimeUTC()), new Date(getToDateTimeUTC())];
	});
</script>

<svelte:window onkeydown={handlePageKeydown} />

<div class="space-y-4">
	<PageHeader title="Dashboards">
		{#snippet actions()}
			{#if dashboards.length > 0}
				<button
					class="inline-flex h-9 items-center gap-2 rounded-md border border-input bg-background px-3 text-sm text-muted-foreground shadow-xs transition-colors hover:bg-muted hover:text-foreground @2xl:w-64"
					onclick={() => (showPalette = true)}
				>
					<Search class="h-4 w-4 shrink-0" />
					<span class="flex-1 truncate text-left">Add or find dashboards...</span>
					<kbd
						class="pointer-events-none hidden h-5 shrink-0 items-center gap-0.5 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium sm:inline-flex"
					>
						{isMac ? '⌘' : 'Ctrl'}K
					</kbd>
				</button>
				<TimeRangePicker
					bind:fromDate
					bind:toDate
					bind:fromTime
					bind:toTime
					bind:preset={selectedPreset}
					onApply={handleTimeRangeChange}
				/>
			{/if}
		{/snippet}
	</PageHeader>

	{#if serverScope}
		<div
			class="flex items-start gap-3 rounded-xl border border-blue-500/20 bg-blue-500/5 px-4 py-3"
		>
			<div
				class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-300"
			>
				<Server class="size-4" />
			</div>
			<div class="min-w-0 flex-1">
				<div class="flex flex-wrap items-center gap-2 text-sm font-medium">
					<span>Instance view</span>
					<code class="rounded bg-background/80 px-1.5 py-0.5 text-xs">{serverScope}</code>
				</div>
				<p class="mt-0.5 text-xs text-muted-foreground">
					Every widget is scoped to this instance. Clear the scope to compare the full project.
				</p>
			</div>
			<Button
				variant="ghost"
				size="icon-sm"
				onclick={clearServerScope}
				title="Clear instance scope"
			>
				<X class="size-4" />
			</Button>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<LoadingCircle size="xlg" />
		</div>
	{:else if listError}
		<ErrorDisplay
			status={400}
			title="Error"
			description={listError}
			onRetry={() => loadDashboards()}
		/>
	{:else if dashboards.length === 0}
		<EmptyState
			message="No dashboards yet."
			actionLabel="Browse Dashboards & Templates"
			actionVariant="default"
			onAction={() => (showPalette = true)}
		>
			{#snippet icon()}
				<Search class="h-6 w-6 text-muted-foreground" />
			{/snippet}
		</EmptyState>
	{:else}
		<Tabs.Root value={activeTabId} onValueChange={handleTabChange}>
			<div class="flex items-center gap-2" bind:this={tabsRowEl}>
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
							{#each dashboards as dashboard (dashboard.id)}
								<Select.Item value={String(dashboard.id)}>{dashboard.name}</Select.Item>
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
						{#each dashboards as dashboard, i (dashboard.id)}
							<Tabs.Trigger
								value={String(dashboard.id)}
								draggable={dashboards.length > 1}
								ondragstart={(e) => handleTabDragStart(e, i)}
								ondragover={(e) => handleTabDragOver(e, i)}
								ondrop={(e) => handleTabDrop(e, i)}
								ondragend={handleTabDragEnd}
								class="transition-opacity {tabDragIndex === i ? 'opacity-40' : ''} {tabDropIndex ===
									i &&
								tabDragIndex !== null &&
								tabDragIndex !== i
									? 'ring-2 ring-primary'
									: ''}"
							>
								{dashboard.name}
								{#if String(dashboard.id) === activeTabId}
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
												Rename Dashboard
											</DropdownMenu.Item>
											<DropdownMenu.Item onclick={openJsonDialog}>
												<Braces class="mr-2 h-4 w-4" />
												Edit as JSON
											</DropdownMenu.Item>
											<DropdownMenu.Item onclick={exportDashboard}>
												<Download class="mr-2 h-4 w-4" />
												Export JSON
											</DropdownMenu.Item>
											<DropdownMenu.Item onclick={openApplyDialog}>
												<Share2 class="mr-2 h-4 w-4" />
												Apply to Projects
											</DropdownMenu.Item>
											<DropdownMenu.Separator />
											<DropdownMenu.Item onclick={() => (showRemoveDialog = true)}>
												<CircleMinus class="mr-2 h-4 w-4" />
												Remove from this Project
											</DropdownMenu.Item>
											<DropdownMenu.Item
												class="text-destructive"
												onclick={() => (showDeleteDialog = true)}
											>
												<Trash2 class="mr-2 h-4 w-4" />
												Delete Dashboard
											</DropdownMenu.Item>
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								{/if}
							</Tabs.Trigger>
						{/each}
					</Tabs.List>
				</div>
				<button
					bind:this={plusBtnEl}
					class="inline-flex items-center justify-center rounded-md px-2 py-1 text-muted-foreground transition-all hover:bg-muted hover:text-foreground"
					title="Add dashboard (⌘K)"
					onclick={() => (showPalette = true)}
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
								Rename Dashboard
							</DropdownMenu.Item>
							<DropdownMenu.Item onclick={openJsonDialog}>
								<Braces class="mr-2 h-4 w-4" />
								Edit as JSON
							</DropdownMenu.Item>
							<DropdownMenu.Item onclick={exportDashboard}>
								<Download class="mr-2 h-4 w-4" />
								Export JSON
							</DropdownMenu.Item>
							<DropdownMenu.Item onclick={openApplyDialog}>
								<Share2 class="mr-2 h-4 w-4" />
								Apply to Projects
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
							<DropdownMenu.Item onclick={() => (showRemoveDialog = true)}>
								<CircleMinus class="mr-2 h-4 w-4" />
								Remove from this Project
							</DropdownMenu.Item>
							<DropdownMenu.Item class="text-destructive" onclick={() => (showDeleteDialog = true)}>
								<Trash2 class="mr-2 h-4 w-4" />
								Delete Dashboard
							</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				{/if}
				{#if lastUpdated}
					<div class="ml-auto flex items-center gap-1" bind:this={updatedEl}>
						<span class="text-sm whitespace-nowrap text-muted-foreground">
							Updated: {lastUpdatedFormatted}
						</span>
						<Button
							variant="ghost"
							size="sm"
							onclick={reloadDashboardWidgets}
							disabled={loadingDashboard}
						>
							<RefreshCw class="h-4 w-4 {loadingDashboard ? 'animate-spin' : ''}" />
						</Button>
					</div>
				{/if}
			</div>

			{#each dashboards as dashboard (dashboard.id)}
				<Tabs.Content value={String(dashboard.id)}>
					{#if loadingDashboard && activeTabId === String(dashboard.id)}
						<div class="flex items-center justify-center py-20">
							<LoadingCircle size="xlg" />
						</div>
					{:else if loadError && activeTabId === String(dashboard.id)}
						<div
							class="flex flex-col items-center justify-center gap-3 rounded-md bg-muted py-20 text-center"
						>
							<p class="text-muted-foreground">{loadError}</p>
							<Button variant="outline" onclick={() => loadDashboardWidgets(activeTabId)}>
								<RefreshCw class="mr-2 h-4 w-4" />
								Retry
							</Button>
						</div>
					{:else if activeDashboard && activeDashboard.id === dashboard.id && activeTabId === String(dashboard.id)}
						<WidgetGrid
							widgets={activeDashboard.widgets ?? []}
							fromDateUTC={getFromDateTimeUTC()}
							toDateUTC={getToDateTimeUTC()}
							timeDomain={sharedTimeDomain}
							{scopeTagFilters}
							onEditWidget={(w) => openEditWidget(w as Widget)}
							onDeleteWidget={(w) => openDeleteWidgetDialog(w as Widget)}
							onReorderWidgets={handleReorderWidgets}
							onDuplicateWidget={(w) => handleDuplicateWidget(w as Widget)}
							onResizeWidget={(w, layout) => handleResizeWidget(w as Widget, layout)}
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
	widget={editingWidget ?? widgetPrefill}
	{availableMetrics}
	error={widgetConfigError}
	onSave={handleWidgetSave}
	onCancel={() => {
		showWidgetConfig = false;
		editingWidget = null;
		widgetPrefill = null;
		widgetConfigError = '';
	}}
/>

<DashboardCommand
	bind:open={showPalette}
	onCreateNew={() => (showCreateDialog = true)}
	onImportJson={() => (showImportDialog = true)}
	onImportGrafana={() => (showGrafanaDialog = true)}
	onApplied={handlePaletteApplied}
	onAddMetric={openAddWidgetWithMetric}
/>

<AlertDialog.Root
	bind:open={showCreateDialog}
	onOpenChange={(o) => {
		if (!o) createError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>New Dashboard</AlertDialog.Title>
		</AlertDialog.Header>
		<ErrorAlert error={createError} />
		<FormField label="Name" for="new-dashboard-name">
			<Input
				id="new-dashboard-name"
				bind:value={newName}
				placeholder="My Dashboard"
				maxlength={100}
			/>
		</FormField>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showCreateDialog = false)} disabled={creating}>
				Cancel
			</Button>
			<Button variant="success" onclick={createDashboard} disabled={creating}>
				{#if creating}Creating...{:else}<Plus class="mr-2 h-4 w-4" /> New Dashboard{/if}
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
			<AlertDialog.Title>Rename Dashboard</AlertDialog.Title>
		</AlertDialog.Header>
		<ErrorAlert error={renameError} />
		<FormField label="Name" for="rename-dashboard-name">
			<Input
				id="rename-dashboard-name"
				bind:value={renameName}
				placeholder="My Dashboard"
				maxlength={100}
			/>
		</FormField>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showRenameDialog = false)} disabled={renaming}>
				Cancel
			</Button>
			<Button onclick={renameDashboard} disabled={renaming}>
				{#if renaming}Updating...{:else}<Check class="mr-2 h-4 w-4" /> Update Dashboard{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showJsonDialog}
	onOpenChange={(o) => {
		if (!o) jsonError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close" class="sm:max-w-3xl">
		<AlertDialog.Header>
			<AlertDialog.Title>Edit as JSON</AlertDialog.Title>
			<AlertDialog.Description>
				The full dashboard definition. Widget order in the array is the display order.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={jsonError} />
		<div class="space-y-4">
			<textarea
				bind:value={jsonText}
				spellcheck="false"
				class="h-96 w-full resize-y rounded-md border border-input bg-transparent p-3 font-mono text-xs shadow-xs focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
			></textarea>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showJsonDialog = false)} disabled={jsonSaving}>
				Cancel
			</Button>
			<Button onclick={saveJson} disabled={jsonSaving}>
				{#if jsonSaving}Updating...{:else}<Check class="mr-2 h-4 w-4" /> Update Dashboard{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showApplyDialog}
	onOpenChange={(o) => {
		if (!o) applyError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Apply to Projects</AlertDialog.Title>
			<AlertDialog.Description>
				Choose which projects show "{activeDashboard?.name}" in their dashboard tabs.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<WarningCallout title="Shared across projects">
			This is a single shared dashboard. Checking a project shows it in that project's tabs. Nothing
			is copied or overwritten, but edits made from any project apply to all of them. Unchecking a
			project removes the dashboard from its tabs, including any of its widgets starred on that
			project's homepage.
		</WarningCallout>
		<ErrorAlert error={applyError} />
		<div class="max-h-72 space-y-2 overflow-y-auto">
			{#each orgProjects as project (project.id)}
				<label class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted">
					<Checkbox
						checked={applySelection[project.id] ?? false}
						onCheckedChange={(checked) => {
							applySelection = { ...applySelection, [project.id]: checked === true };
						}}
					/>
					<span class="text-sm">{project.name}</span>
					{#if project.id === projectsState.currentProjectId}
						<span class="text-xs text-muted-foreground">(current)</span>
					{/if}
				</label>
			{/each}
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showApplyDialog = false)} disabled={applying}>
				Cancel
			</Button>
			<Button onclick={saveApply} disabled={applying}>
				{#if applying}Applying...{:else}<Check class="mr-2 h-4 w-4" /> Update Dashboard{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showImportDialog}
	onOpenChange={(o) => {
		if (!o) importError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close" class="sm:max-w-2xl">
		<AlertDialog.Header>
			<AlertDialog.Title>Import Dashboards</AlertDialog.Title>
			<AlertDialog.Description>
				Paste an exported dashboard JSON document or bundle. Imported dashboards are applied to the
				current project.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={importError} />
		<div class="space-y-2">
			<textarea
				bind:value={importText}
				spellcheck="false"
				placeholder={'{"schemaVersion": 1, "name": "...", "widgets": [...]}'}
				class="h-64 w-full resize-y rounded-md border border-input bg-transparent p-3 font-mono text-xs shadow-xs focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
			></textarea>
			<Button variant="outline" size="sm" onclick={() => readFileInto((t) => (importText = t))}>
				<Upload class="mr-2 h-4 w-4" />
				Load from file
			</Button>
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showImportDialog = false)} disabled={importing}>
				Cancel
			</Button>
			<Button variant="success" onclick={importDashboards} disabled={importing}>
				{#if importing}Importing...{:else}<Plus class="mr-2 h-4 w-4" /> Import Dashboards{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showGrafanaDialog}
	onOpenChange={(o) => {
		if (!o) {
			grafanaError = '';
			grafanaWarnings = [];
		}
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close" class="sm:max-w-2xl">
		<AlertDialog.Header>
			<AlertDialog.Title>Import from Grafana</AlertDialog.Title>
			<AlertDialog.Description>
				Paste a Grafana dashboard JSON export. Panels and Prometheus queries are converted on a
				best-effort basis.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={grafanaError} />
		{#if grafanaWarnings.length > 0}
			<WarningCallout title="Imported with warnings">
				<div class="max-h-48 space-y-1 overflow-y-auto">
					{#each grafanaWarnings as warning, __index (__index)}
						<div class="text-xs">{warning}</div>
					{/each}
				</div>
			</WarningCallout>
		{:else}
			<div class="space-y-2">
				<textarea
					bind:value={grafanaText}
					spellcheck="false"
					placeholder={'{"title": "...", "panels": [...]}'}
					class="h-64 w-full resize-y rounded-md border border-input bg-transparent p-3 font-mono text-xs shadow-xs focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
				></textarea>
				<Button variant="outline" size="sm" onclick={() => readFileInto((t) => (grafanaText = t))}>
					<Upload class="mr-2 h-4 w-4" />
					Load from file
				</Button>
			</div>
		{/if}
		<AlertDialog.Footer>
			{#if grafanaWarnings.length > 0}
				<Button onclick={() => (showGrafanaDialog = false)}>
					<Check class="mr-2 h-4 w-4" /> Done
				</Button>
			{:else}
				<Button
					variant="outline"
					onclick={() => (showGrafanaDialog = false)}
					disabled={grafanaImporting}
				>
					Cancel
				</Button>
				<Button variant="success" onclick={importGrafana} disabled={grafanaImporting}>
					{#if grafanaImporting}Importing...{:else}<Plus class="mr-2 h-4 w-4" /> Import Dashboard{/if}
				</Button>
			{/if}
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<AlertDialog.Root
	bind:open={showRemoveDialog}
	onOpenChange={(o) => {
		if (!o) removeError = '';
	}}
>
	<AlertDialog.Content interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>Remove Dashboard</AlertDialog.Title>
			<AlertDialog.Description>
				Remove "{activeDashboard?.name}" from this project's tabs? The dashboard itself is kept and
				stays available in other projects and the library.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={removeError} />
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showRemoveDialog = false)} disabled={removing}>
				Cancel
			</Button>
			<Button variant="destructive" onclick={removeFromProject} disabled={removing}>
				<Trash2 class="mr-2 h-4 w-4" />
				{removing ? 'Removing...' : 'Remove Dashboard'}
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
			<AlertDialog.Title>Delete Dashboard</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete "{activeDashboard?.name}"? It will be removed from every
				project it is applied to{activeDashboard && activeDashboard.appliedProjectIds.length > 1
					? ` (${activeDashboard.appliedProjectIds.length} projects)`
					: ''}, along with all of its widgets. This action cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<ErrorAlert error={deleteError} />
		<AlertDialog.Footer>
			<Button variant="outline" onclick={() => (showDeleteDialog = false)} disabled={deleting}>
				Cancel
			</Button>
			<Button variant="destructive" onclick={deleteDashboard} disabled={deleting}>
				<Trash2 class="mr-2 h-4 w-4" />
				{deleting ? 'Deleting...' : 'Delete Dashboard'}
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
		<ErrorAlert error={deleteWidgetError} />
		<AlertDialog.Footer>
			<Button
				variant="outline"
				onclick={() => (showDeleteWidgetDialog = false)}
				disabled={deletingWidgetLoading}>Cancel</Button
			>
			<Button variant="destructive" onclick={confirmDeleteWidget} disabled={deletingWidgetLoading}>
				<Trash2 class="mr-2 h-4 w-4" />
				{deletingWidgetLoading ? 'Deleting...' : 'Delete Widget'}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
