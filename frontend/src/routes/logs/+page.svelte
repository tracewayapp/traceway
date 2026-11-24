<script lang="ts">
	import { getErrorMessage } from '$lib/utils/errors';
	import { onMount, onDestroy, tick } from 'svelte';
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { projectsState } from '$lib/state/projects.svelte';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { SeverityBadge } from '$lib/components/ui/severity-badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import {
		FilterBar,
		type FilterApply,
		type FilterField,
		describeTimeRange,
		attributeConfig,
		traceIdField,
		type AttributeRules
	} from '$lib/components/traceway/filter-bar';
	import ExpandedLogRow from '$lib/components/trace-logs/expanded-log-row.svelte';
	import LogMessage from '$lib/components/trace-logs/log-message.svelte';
	import LogVolumeChart, {
		type LogVolumeBucket
	} from '$lib/components/trace-logs/log-volume-chart.svelte';
	import PageHeader from '$lib/components/traceway/page-header.svelte';
	import TableContainer from '$lib/components/traceway/table-container.svelte';
	import SavedFiltersMenu from '$lib/components/traceway/saved-filters-menu.svelte';
	import ChartPanel, { type ChartSelection } from '$lib/components/value-chart/chart-panel.svelte';
	import { Play } from '@lucide/svelte';
	import {
		filterOperator,
		keyOperatorOptions,
		operatorFlags,
		type AttributeFilter,
		type FilterOperator
	} from '$lib/utils/attribute-filters';
	import {
		filtersEqual,
		formatLogAttributeFilter,
		formatLogAttributeTarget,
		parseLogAttributeFilter,
		parseLogAttributeTarget,
		sameFilterTarget,
		type LogAttributeFilter,
		type LogAttributeTarget
	} from '$lib/utils/log-filters';
	import {
		defaultChartAggregations,
		formatChartParams,
		parseChartParams,
		type LogAttributeChart
	} from '$lib/utils/log-attribute-chart';
	import { paramsToApply, toQueryString, type SavedFilter } from '$lib/utils/saved-filters';
	import { CalendarDate } from '@internationalized/date';
	import {
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		updateUrl
	} from '$lib/utils/url-params';
	import { calendarDateTimeToLuxon, toUTCISO, formatDateTime } from '$lib/utils/formatters';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());
	const initialTimezone = getTimezone();

	const SORT_STORAGE_KEY = 'logs';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'timestamp', direction: 'desc' });
	let sortField = $state(initialSort.field);
	let sortDirection = $state<SortDirection>(initialSort.direction);

	type LogRecord = {
		id: string;
		projectId: string;
		timestamp: string;
		traceId: string;
		spanId: string;
		traceFlags: number;
		severityText: string;
		severityNumber: number;
		serviceName: string;
		body: string;
		resourceSchemaUrl: string;
		resourceAttributes: Record<string, string> | null;
		scopeSchemaUrl: string;
		scopeName: string;
		scopeVersion: string;
		scopeAttributes: Record<string, string> | null;
		logAttributes: Record<string, string> | null;
	};

	let logs = $state<LogRecord[]>([]);
	let loading = $state(true);
	let error = $state('');
	let expandedId = $state<string | null>(null);
	let showAllAttributes = $state(false);

	const pageSize = 50;
	// total counts the fixed history window (lastFromISO..lastToISO) as reported
	// by the backend; liveExtra counts live-prepended rows newer than lastToISO.
	// Keeping them separate stops loadMore's total refresh from erasing the live
	// rows out of "Showing X of Y".
	let total = $state(0);
	let liveExtra = $state(0);
	const displayTotal = $derived(total + liveExtra);
	let loadedPages = 1;
	let loadingMore = $state(false);
	let loadMoreError = $state('');

	type LogsResponse = { data: LogRecord[]; pagination: { total: number; totalPages: number } };

	// Live tail mode: poll only the diff since the last poll and prepend it.
	const LIVE_POLL_MS = 5000;
	// Re-query a little of the already-covered window each poll so records that
	// arrive late (SDK batching) aren't missed; dedup by id drops the overlap.
	const LIVE_OVERLAP_MS = 10_000;
	// Every LIVE_CATCHUP_EVERY-th poll widens the overlap to sweep up records
	// that arrived later than the regular overlap covers (SDK upload retries,
	// OTel batch exporters, client clock skew). Widening only periodically
	// keeps the steady-state poll response small: the wide window is shipped
	// once per sweep interval instead of on every poll, and dedup by id drops
	// everything already loaded.
	const LIVE_CATCHUP_OVERLAP_MS = 60_000;
	const LIVE_CATCHUP_EVERY = 6;
	// Polls fetch a large page so a burst between polls doesn't outrun the
	// window; when even that page is entirely new rows, livePoll gives up on
	// continuity and replaces the buffer (see below).
	const LIVE_PAGE_SIZE = 500;
	// Cap the buffer so a long-running live session can't grow the DOM forever.
	const MAX_ROWS = 2000;
	let live = $state(false);
	let liveTimer: ReturnType<typeof setInterval> | null = null;
	let pollBusy = false;
	let liveSinceMs = 0;
	let livePollCount = 0;
	// Time range used by the last full load. loadMore and livePoll reuse it so
	// page offsets stay stable while the wall clock moves.
	let lastFromISO = '';
	let lastToISO = '';
	let lastFilterBody: ReturnType<typeof currentFilterBody> | null = null;
	let requestGen = 0;

	type HistogramResponse = { intervalSeconds: number; total: number; buckets: LogVolumeBucket[] };
	let volume = $state<HistogramResponse | null>(null);
	let volumeLoading = $state(false);
	let volumeEl = $state<HTMLDivElement | null>(null);
	let volumeGen = 0;
	// Charts only show for a search that found something.
	const hasResults = $derived(
		(loading || !!error || logs.length > 0) && !(volume?.total === 0 && !volumeLoading)
	);

	// Windowed rendering: only rows near the viewport are mounted, spacer rows
	// keep the scrollbar honest. Row height is measured from the first rendered
	// row; the (single) expanded row's extra height is measured separately and
	// folded into the offset math piecewise. INVARIANT: every collapsed row must
	// render at the same height — all cells truncate to one line today; a cell
	// that wraps would desync the spacer math and make the scrollbar drift.
	const OVERSCAN = 10;
	let scrollEl = $state<HTMLDivElement | null>(null);
	let scrollTop = $state(0);
	let viewportH = $state(600);
	let rowH = $state(44);
	let expandedHeight = $state(0);

	const expandedIndex = $derived(expandedId ? logs.findIndex((l) => l.id === expandedId) : -1);

	function offsetOf(i: number): number {
		return i * rowH + (expandedIndex >= 0 && i > expandedIndex ? expandedHeight : 0);
	}

	function indexAt(y: number): number {
		if (expandedIndex < 0) return Math.floor(y / rowH);
		const afterExpandedTop = (expandedIndex + 1) * rowH + expandedHeight;
		if (y < (expandedIndex + 1) * rowH) return Math.floor(y / rowH);
		if (y < afterExpandedTop) return expandedIndex;
		return expandedIndex + 1 + Math.floor((y - afterExpandedTop) / rowH);
	}

	const winStart = $derived(Math.max(0, indexAt(scrollTop) - OVERSCAN));
	const winEnd = $derived(Math.min(logs.length, indexAt(scrollTop + viewportH) + 1 + OVERSCAN));
	const topPad = $derived(offsetOf(winStart));
	const bottomPad = $derived(Math.max(0, offsetOf(logs.length) - offsetOf(winEnd)));
	const visibleLogs = $derived(logs.slice(winStart, winEnd));

	// Size the scroll container so the table ends at the bottom of the viewport
	// instead of overflowing the page. max-height (not height) so a short result
	// list doesn't leave a stretched empty table.
	function updateMaxHeight() {
		if (!scrollEl) return;
		const top = scrollEl.getBoundingClientRect().top;
		scrollEl.style.maxHeight = `${Math.max(240, window.innerHeight - top - 24)}px`;
	}

	$effect(() => {
		// Anything that can move the table's top edge (wrapping filter pills,
		// loading states) should trigger a re-measure.
		void filterValues;
		void attributeFilters.length;
		void loading;
		void volume;
		void chart;
		updateMaxHeight();
	});

	$effect(() => {
		void logs.length;
		if (!scrollEl) return;
		const first = scrollEl.querySelector('tr[data-log-id]') as HTMLElement | null;
		if (first && first.offsetHeight > 0) rowH = first.offsetHeight;
	});

	// Reported by the expanded row whenever it resizes. Its height is kept while
	// the row is scrolled out of the rendered window, so the spacers stay right.
	function setExpandedHeight(height: number) {
		expandedHeight = height;
	}

	function parseLogsUrlParams(search?: URLSearchParams) {
		if (!browser && !search) {
			return {
				preset: '24h',
				from: null,
				to: null,
				search: '',
				searchType: 'body',
				minSeverity: 0,
				serviceName: '',
				traceId: '',
				spanId: '',
				scopeName: '',
				body: '',
				attributeFilters: [] as LogAttributeFilter[],
				chart: null as LogAttributeChart | null
			};
		}
		const params = search ?? new URLSearchParams(window.location.search);
		const timeParams = parseTimeRangeFromUrl(timezone, '24h', params);
		const minSev = Number(params.get('minSeverity') || '0');
		const attrs: LogAttributeFilter[] = [];
		for (const raw of params.getAll('attr')) {
			const parsed = parseLogAttributeFilter(raw);
			if (parsed) attrs.push(parsed);
		}
		return {
			...timeParams,
			search: params.get('search') || '',
			searchType: params.get('searchType') || 'body',
			minSeverity: Number.isFinite(minSev) ? minSev : 0,
			serviceName: params.get('service') || '',
			traceId: params.get('traceId') || '',
			spanId: params.get('spanId') || '',
			scopeName: params.get('scopeName') || '',
			body: params.get('body') || '',
			attributeFilters: attrs,
			chart: parseChartParams(params)
		};
	}

	const initialUrlParams = parseLogsUrlParams();
	const initialRange = getResolvedTimeRange(initialUrlParams, initialTimezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, initialTimezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, initialTimezone));
	let fromTime = $state(dateToTimeString(initialRange.from, initialTimezone));
	let toTime = $state(dateToTimeString(initialRange.to, initialTimezone));

	let searchQuery = $state(initialUrlParams.search);
	let searchType = $state(initialUrlParams.searchType);
	let minSeverity = $state<number>(initialUrlParams.minSeverity);
	let serviceName = $state(initialUrlParams.serviceName);
	let traceIdFilter = $state(initialUrlParams.traceId);
	let spanIdFilter = $state(initialUrlParams.spanId);
	let scopeNameFilter = $state(initialUrlParams.scopeName);
	let bodyFilter = $state(initialUrlParams.body);
	let attributeFilters = $state<LogAttributeFilter[]>(initialUrlParams.attributeFilters);
	let chart = $state<LogAttributeChart | null>(initialUrlParams.chart);
	// What the list shows; the chart reloads when it changes.
	let chartSelection = $state<ChartSelection | null>(null);
	// The applied filters as a query string, for the saved filters menu.
	let appliedQuery = $state('');

	const searchTypeOptions = [
		{ value: 'body', label: 'Message' },
		{ value: 'service', label: 'Service' },
		{ value: 'trace', label: 'Trace ID' }
	];

	const severityOptions = [
		{ value: '1', label: 'TRACE+', pill: 'Level ≥ TRACE' },
		{ value: '5', label: 'DEBUG+', pill: 'Level ≥ DEBUG' },
		{ value: '9', label: 'INFO+', pill: 'Level ≥ INFO' },
		{ value: '13', label: 'WARN+', pill: 'Level ≥ WARN' },
		{ value: '17', label: 'ERROR+', pill: 'Level ≥ ERROR' },
		{ value: '21', label: 'FATAL', pill: 'Level = FATAL' }
	];

	const filterFields: FilterField[] = [
		{
			kind: 'select',
			key: 'searchType',
			label: 'Search in',
			anyLabel: 'Message',
			emptyValue: 'body',
			options: searchTypeOptions,
			inSearch: true
		},
		{
			kind: 'select',
			key: 'minSeverity',
			label: 'Level',
			anyLabel: 'Any level',
			emptyValue: '0',
			options: severityOptions,
			inSearch: true
		},
		{
			kind: 'text',
			key: 'serviceName',
			label: 'Service',
			placeholder: 'Exact service name',
			hint: 'The exact service name.'
		},
		traceIdField('traceId', 'Every log of one trace.'),
		{
			kind: 'text',
			key: 'spanId',
			label: 'Span ID',
			placeholder: '16 hex characters',
			mono: true
		},
		{
			kind: 'text',
			key: 'scopeName',
			label: 'Scope',
			placeholder: 'Instrumentation scope name',
			hint: 'The exact scope name.'
		},
		{
			kind: 'text',
			key: 'body',
			label: 'Exact message',
			placeholder: 'The whole message',
			hint: 'Matches the whole message exactly. Set it from a log row.'
		}
	];

	const filterValues = $derived({
		searchType,
		minSeverity: String(minSeverity),
		serviceName,
		traceId: traceIdFilter,
		spanId: spanIdFilter,
		scopeName: scopeNameFilter,
		body: bodyFilter
	});

	// FilterBar edits flat keys (`resource.service.name`); the page keeps the scope apart.
	function toFlatFilter(f: LogAttributeFilter): AttributeFilter {
		return {
			key: formatLogAttributeTarget(f),
			value: f.value,
			exclude: f.exclude,
			contains: f.contains,
			exists: f.exists
		};
	}

	function fromFlatFilter(f: AttributeFilter): LogAttributeFilter[] {
		const target = parseLogAttributeTarget(f.key);
		return target ? [{ ...f, ...target }] : [];
	}

	const LOG_ATTRIBUTE_RULES: AttributeRules = {
		operators: keyOperatorOptions,
		keyPlaceholder: 'resource.service.name',
		valuePlaceholder: 'backend-service',
		value: 'trim-end',
		key: (key) => {
			const target = parseLogAttributeTarget(key);
			return target && target.key.trim()
				? formatLogAttributeTarget(target)
				: { error: 'Key must start with resource., scope., or log.' };
		}
	};

	const attributeFilterConfig = $derived(
		attributeConfig(attributeFilters.map(toFlatFilter), LOG_ATTRIBUTE_RULES)
	);

	function handleFilterApply(next: FilterApply) {
		searchQuery = next.search;
		searchType = next.values.searchType || 'body';
		minSeverity = Number(next.values.minSeverity) || 0;
		serviceName = next.values.serviceName ?? '';
		traceIdFilter = next.values.traceId ?? '';
		spanIdFilter = next.values.spanId ?? '';
		scopeNameFilter = next.values.scopeName ?? '';
		bodyFilter = next.values.body ?? '';
		attributeFilters = next.attributes.flatMap(fromFlatFilter);
		loadData(true);
	}

	function updateAttributeFilter(index: number, filter: LogAttributeFilter) {
		const dup = attributeFilters.some((f, i) => i !== index && filtersEqual(f, filter));
		attributeFilters = dup
			? attributeFilters.filter((_, i) => i !== index)
			: attributeFilters.map((f, i) => (i === index ? filter : f));
		loadData(true);
	}

	function addAttributeFilter(filter: LogAttributeFilter) {
		if (!attributeFilters.some((f) => filtersEqual(f, filter))) {
			attributeFilters = [...attributeFilters, filter];
		}
		loadData(true);
	}

	// An attribute tile reflects the filters on its own key and value, whatever
	// their operator, or else an existence filter on its key. A contains filter
	// typed with a substring does not match.
	function onTile(f: LogAttributeFilter, target: LogAttributeTarget, value: string): boolean {
		return f.exists
			? f.scope === target.scope && f.key === target.key
			: sameFilterTarget(f, { ...target, value });
	}

	function tileFilterIndex(target: LogAttributeTarget, value: string): number {
		const byValue = attributeFilters.findIndex((f) => !f.exists && onTile(f, target, value));
		return byValue !== -1 ? byValue : attributeFilters.findIndex((f) => onTile(f, target, value));
	}

	function attributeFilterOperator(
		target: LogAttributeTarget,
		value: string
	): FilterOperator | null {
		const index = tileFilterIndex(target, value);
		return index === -1 ? null : filterOperator(attributeFilters[index]);
	}

	// From the operator menu of an attribute tile. Picking the active operator
	// again removes the filter, picking another one changes it in place, and
	// null clears every filter the tile shows.
	function setAttributeFilterOperator(
		target: LogAttributeTarget,
		value: string,
		operator: FilterOperator | null
	) {
		const index = tileFilterIndex(target, value);
		if (operator === null) {
			attributeFilters = attributeFilters.filter((f) => !onTile(f, target, value));
			loadData(true);
			return;
		}
		const flags = operatorFlags(operator);
		const filter: LogAttributeFilter = { ...target, value: flags.exists ? '' : value, ...flags };
		if (index === -1) {
			addAttributeFilter(filter);
		} else if (filterOperator(attributeFilters[index]) === operator) {
			removeAttributeFilter(index);
		} else {
			updateAttributeFilter(index, filter);
		}
	}

	function openChart(target: LogAttributeTarget) {
		const same = chart && chart.target.scope === target.scope && chart.target.key === target.key;
		if (!same) {
			chart = { target, view: 'series', aggregations: [...defaultChartAggregations] };
			updateLogsUrl(true);
		}
		tick().then(() => volumeEl?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }));
	}

	// View and line changes replace the history entry instead of adding one per click.
	function setChart(next: LogAttributeChart) {
		chart = next;
		updateLogsUrl(false);
	}

	function closeChart() {
		chart = null;
		updateLogsUrl(true);
	}

	const LOG_CHART_ENDPOINTS = {
		series: '/logs/attribute-series',
		distribution: '/logs/attribute-distribution'
	};

	// Filters for the fixed tiles of the expanded row (Body / Trace ID /
	// Span ID / Scope), backed by their dedicated query params rather than
	// attribute filters.
	type MetaFilterField = 'body' | 'traceId' | 'spanId' | 'scopeName';

	function isMetaFilterActive(field: MetaFilterField, value: string): boolean {
		switch (field) {
			case 'body':
				return bodyFilter === value;
			case 'traceId':
				return traceIdFilter === value;
			case 'spanId':
				return spanIdFilter === value;
			case 'scopeName':
				return scopeNameFilter === value;
		}
	}

	function toggleMetaFilter(field: MetaFilterField, value: string) {
		const active = isMetaFilterActive(field, value);
		switch (field) {
			case 'body':
				bodyFilter = active ? '' : value;
				break;
			case 'traceId':
				traceIdFilter = active ? '' : value;
				break;
			case 'spanId':
				spanIdFilter = active ? '' : value;
				break;
			case 'scopeName':
				scopeNameFilter = active ? '' : value;
				break;
		}
		loadData(true);
	}

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

	function logsUrlParams(): Record<string, string | string[] | null> {
		return {
			preset: selectedPreset || null,
			from: selectedPreset ? null : getFromDateTimeUTC(),
			to: selectedPreset ? null : getToDateTimeUTC(),
			search: searchQuery.trim() || null,
			searchType: searchType !== 'body' ? searchType : null,
			minSeverity: minSeverity > 0 ? String(minSeverity) : null,
			service: serviceName.trim() || null,
			traceId: traceIdFilter.trim() || null,
			spanId: spanIdFilter.trim() || null,
			scopeName: scopeNameFilter.trim() || null,
			body: bodyFilter || null,
			attr: attributeFilters.map(formatLogAttributeFilter),
			...formatChartParams(chart)
		};
	}

	function updateLogsUrl(pushToHistory = true) {
		const params = logsUrlParams();
		appliedQuery = toQueryString(params);
		updateUrl(params, { pushToHistory });
	}

	const timeRangeLabel = $derived(describeTimeRange(selectedPreset));

	function removeAttributeFilter(index: number) {
		attributeFilters = attributeFilters.filter((_, i) => i !== index);
		loadData(true);
	}

	function currentFilterBody() {
		return {
			search: searchQuery.trim(),
			searchType,
			minSeverity,
			serviceName: serviceName.trim(),
			traceId: traceIdFilter.trim(),
			spanId: spanIdFilter.trim(),
			scopeName: scopeNameFilter.trim(),
			body: bodyFilter,
			attributeFilters
		};
	}

	async function loadData(pushToHistory = true) {
		const gen = ++requestGen;
		loading = true;
		error = '';
		loadMoreError = '';
		loadedPages = 1;

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		updateLogsUrl(pushToHistory);

		try {
			lastFromISO = getFromDateTimeUTC();
			lastToISO = getToDateTimeUTC();
			lastFilterBody = currentFilterBody();
			const requestBody = {
				fromDate: lastFromISO,
				toDate: lastToISO,
				orderBy: sortField,
				sortDirection,
				pagination: { page: 1, pageSize },
				...lastFilterBody
			};

			loadVolume(lastFromISO, lastToISO, lastFilterBody);
			chartSelection = { fromDate: lastFromISO, toDate: lastToISO, ...lastFilterBody };
			const response = (await api.post('/logs', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			})) as LogsResponse;

			if (gen !== requestGen) return;
			logs = response.data || [];
			total = response.pagination.total;
			liveExtra = 0;
			scrollEl?.scrollTo({ top: 0 });
			scrollTop = 0;
		} catch (e) {
			if (gen !== requestGen) return;
			console.error(e);
			error = getErrorMessage(e) || 'Failed to load logs';
		} finally {
			if (gen === requestGen) loading = false;
		}
	}

	// The volume strip loads beside the list rather than before it: a slow count
	// over a wide range must never hold back the rows.
	async function loadVolume(
		fromISO: string,
		toISO: string,
		filters: ReturnType<typeof currentFilterBody>
	) {
		const gen = ++volumeGen;
		volumeLoading = true;
		try {
			const maxBuckets = Math.max(
				20,
				Math.min(120, Math.floor((volumeEl?.clientWidth || 960) / 12))
			);
			const response = (await api.post(
				'/logs/histogram',
				{ fromDate: fromISO, toDate: toISO, maxBuckets, ...filters },
				{ projectId: projectsState.currentProjectId ?? undefined }
			)) as HistogramResponse;
			if (gen !== volumeGen) return;
			volume = response;
		} catch (e) {
			if (gen !== volumeGen) return;
			console.error(e);
			volume = null;
		} finally {
			if (gen === volumeGen) volumeLoading = false;
		}
	}

	// Live tail slides the strip's window along with the clock every few polls,
	// instead of recounting the whole range every 5 seconds.
	function refreshLiveVolume() {
		if (!selectedPreset) return;
		const range = getTimeRangeFromPreset(selectedPreset, timezone);
		const filters = lastFilterBody ?? currentFilterBody();
		loadVolume(range.from.toISOString(), range.to.toISOString(), filters);
		if (chart) {
			chartSelection = {
				fromDate: range.from.toISOString(),
				toDate: range.to.toISOString(),
				...filters
			};
		}
	}

	function handleVolumeRangeSelect(from: Date, to: Date) {
		handleTimeRangeChange(
			{ date: dateToCalendarDate(from, timezone), time: dateToTimeString(from, timezone) },
			{ date: dateToCalendarDate(to, timezone), time: dateToTimeString(to, timezone) },
			null
		);
	}

	async function loadMore() {
		if (loadingMore || loading) return;
		const gen = requestGen;
		loadingMore = true;
		loadMoreError = '';
		try {
			const requestBody = {
				fromDate: lastFromISO,
				toDate: lastToISO,
				orderBy: sortField,
				sortDirection,
				pagination: { page: loadedPages + 1, pageSize },
				...(lastFilterBody ?? currentFilterBody())
			};

			const response = (await api.post('/logs', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			})) as LogsResponse;

			if (gen !== requestGen) return;
			// Live-prepended rows shift page offsets, so drop anything already loaded.
			const seen = new Set(logs.map((l) => l.id));
			const fresh = (response.data || []).filter((l) => !seen.has(l.id));
			logs = [...logs, ...fresh];
			loadedPages += 1;
			total = response.pagination.total;
		} catch (e) {
			console.error(e);
			loadMoreError = getErrorMessage(e) || 'Failed to load more logs';
		} finally {
			loadingMore = false;
		}
	}

	async function livePoll() {
		if (pollBusy || loading || document.hidden) return;
		const gen = requestGen;
		pollBusy = true;
		try {
			const nowMs = Date.now();
			livePollCount += 1;
			const overlap =
				livePollCount % LIVE_CATCHUP_EVERY === 0 ? LIVE_CATCHUP_OVERLAP_MS : LIVE_OVERLAP_MS;
			const toISO = new Date(nowMs).toISOString();
			const response = (await api.post(
				'/logs',
				{
					fromDate: new Date(liveSinceMs - overlap).toISOString(),
					toDate: toISO,
					orderBy: 'timestamp',
					sortDirection: 'desc',
					pagination: { page: 1, pageSize: LIVE_PAGE_SIZE },
					...(lastFilterBody ?? currentFilterBody())
				},
				{ projectId: projectsState.currentProjectId ?? undefined }
			)) as LogsResponse;

			if (gen !== requestGen) return;
			const seen = new Set(logs.map((l) => l.id));
			const fresh = (response.data || []).filter((l) => !seen.has(l.id));
			if (fresh.length === LIVE_PAGE_SIZE) {
				// Every returned row is unseen: ingestion outran the poll window
				// and there is a gap between the response and the loaded buffer.
				// Restart the view from the newest rows instead of prepending
				// across a hole. Moving lastToISO up and treating the buffer as
				// the first LIVE_PAGE_SIZE/pageSize pages keeps "Load more"
				// offsets aligned with the new tail. The poll-window total
				// undercounts the widened history window; the next loadMore
				// refreshes it.
				logs = response.data;
				total = response.pagination.total;
				liveExtra = 0;
				lastToISO = toISO;
				loadedPages = LIVE_PAGE_SIZE / pageSize;
				scrollEl?.scrollTo({ top: 0 });
				scrollTop = 0;
			} else if (fresh.length > 0) {
				const merged = [...fresh, ...logs];
				if (merged.length > MAX_ROWS) {
					// Outside the replace branch the buffer is a contiguous
					// newest-first run, so after truncation it is exactly the
					// newest MAX_ROWS of (lastFromISO, toISO]. Re-anchor the
					// "Load more" window to that tail so the next page follows
					// it instead of leaving a hole where the dropped rows were.
					logs = merged.slice(0, MAX_ROWS);
					lastToISO = toISO;
					loadedPages = MAX_ROWS / pageSize;
					total += liveExtra + fresh.length;
					liveExtra = 0;
				} else {
					logs = merged;
					liveExtra += fresh.length;
				}
				// Keep the rows the user is looking at pinned in place while
				// new rows grow the list above them; at the very top, stay
				// pinned to the newest row instead.
				if (scrollTop > 0 && scrollEl) {
					const target = scrollTop + fresh.length * rowH;
					tick().then(() => {
						if (!scrollEl) return;
						scrollEl.scrollTop = target;
						scrollTop = scrollEl.scrollTop;
					});
				}
			}
			liveSinceMs = nowMs;
			if (livePollCount % LIVE_CATCHUP_EVERY === 0) refreshLiveVolume();
		} catch (e) {
			console.error(e);
		} finally {
			pollBusy = false;
		}
	}

	function toggleLive() {
		if (live) {
			stopLive();
			return;
		}
		// Live tail only makes sense newest-first: clear any custom sort.
		if (sortField !== 'timestamp' || sortDirection !== 'desc') {
			sortField = 'timestamp';
			sortDirection = 'desc';
			setSortState(SORT_STORAGE_KEY, { field: sortField, direction: sortDirection });
		}
		live = true;
		liveSinceMs = Date.now();
		livePollCount = 0;
		loadData(true);
		liveTimer = setInterval(livePoll, LIVE_POLL_MS);
	}

	function stopLive() {
		live = false;
		if (liveTimer) {
			clearInterval(liveTimer);
			liveTimer = null;
		}
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
		// A custom range is a fixed window in the past; there is nothing to tail.
		if (!preset && live) stopLive();
		loadData(true);
	}

	function handleSort(field: string) {
		// Live tail is pinned to newest-first; sorting drops back to history mode.
		if (live) stopLive();
		const newSort = handleSortClick(field, sortField, sortDirection);
		sortField = newSort.field;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
		loadData(true);
	}

	function handlePopState() {
		applyUrlState(parseLogsUrlParams());
		loadData(false);
	}

	function applySavedFilter(filter: SavedFilter) {
		const params = paramsToApply(filter, {
			preset: selectedPreset,
			from: getFromDateTimeUTC(),
			to: getToDateTimeUTC()
		});
		applyUrlState(parseLogsUrlParams(params));
		loadData(true);
	}

	function applyUrlState(urlParams: ReturnType<typeof parseLogsUrlParams>) {
		const range = getResolvedTimeRange(urlParams, timezone);
		const storedSort = getSortState(SORT_STORAGE_KEY, { field: 'timestamp', direction: 'desc' });
		sortField = storedSort.field;
		sortDirection = storedSort.direction;
		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toTime = dateToTimeString(range.to, timezone);
		searchQuery = urlParams.search;
		searchType = urlParams.searchType;
		minSeverity = urlParams.minSeverity;
		serviceName = urlParams.serviceName;
		traceIdFilter = urlParams.traceId;
		spanIdFilter = urlParams.spanId;
		scopeNameFilter = urlParams.scopeName;
		bodyFilter = urlParams.body;
		attributeFilters = urlParams.attributeFilters;
		chart = urlParams.chart;
		if (!urlParams.preset && live) stopLive();
	}

	function toggleExpanded(id: string) {
		expandedId = expandedId === id ? null : id;
		showAllAttributes = false;
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		window.addEventListener('resize', updateMaxHeight);
		loadData(false);
	});

	onDestroy(() => {
		stopLive();
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
			window.removeEventListener('resize', updateMaxHeight);
		}
	});
</script>

<div class="space-y-4">
	<PageHeader title="Logs">
		{#snippet actions()}
			{#if selectedPreset === null}
				<Tooltip.Root>
					<!-- buttonVariants sets aria-disabled:pointer-events-none, which would
					     swallow hover and prevent this tooltip from ever opening. -->
					<Tooltip.Trigger
						class="{buttonVariants({
							variant: 'outline'
						})} !pointer-events-auto shrink-0 !cursor-not-allowed opacity-50"
						aria-disabled="true"
					>
						<Play class="h-4 w-4" />
						Live
					</Tooltip.Trigger>
					<Tooltip.Content>
						Live follows the current time, so it's unavailable with a custom time range. Pick a
						preset like "24 hours" to tail logs.
					</Tooltip.Content>
				</Tooltip.Root>
			{:else}
				<Button
					variant={live ? 'default' : 'outline'}
					class="shrink-0"
					aria-pressed={live}
					title={live ? 'Stop live tail' : 'Tail new logs in realtime'}
					onclick={toggleLive}
				>
					<Play class="h-4 w-4 {live ? 'fill-current' : ''}" />
					Live
				</Button>
			{/if}
			<TimeRangePicker
				bind:fromDate
				bind:toDate
				bind:fromTime
				bind:toTime
				bind:preset={selectedPreset}
				onApply={handleTimeRangeChange}
			/>
		{/snippet}
	</PageHeader>

	<FilterBar
		search={searchQuery}
		searchPlaceholder="Search logs..."
		fields={filterFields}
		values={filterValues}
		attributes={attributeFilterConfig}
		onApply={handleFilterApply}
		disabled={loading}
		noun="logs"
		rangeLabel={timeRangeLabel}
	>
		{#snippet actions()}
			<SavedFiltersMenu
				page="logs"
				currentQuery={appliedQuery}
				{timeRangeLabel}
				onApply={applySavedFilter}
			/>
		{/snippet}

		{@render results()}
	</FilterBar>

	{#snippet results()}
		<TableContainer>
			<div bind:this={volumeEl}>
				{#if !hasResults}
					<!-- An empty search draws no chart: a flat strip of empty buckets says nothing. -->
				{:else if volume}
					<LogVolumeChart
						buckets={volume.buckets}
						intervalSeconds={volume.intervalSeconds}
						total={volume.total}
						{timezone}
						loading={volumeLoading}
						onRangeSelect={handleVolumeRangeSelect}
					/>
				{:else}
					<div
						class="flex h-[95px] items-center justify-center border-b text-xs text-muted-foreground"
					>
						{#if volumeLoading}
							<LoadingCircle size="sm" />
						{:else}
							Log volume is unavailable for this search.
						{/if}
					</div>
				{/if}
			</div>
			{#if chart && hasResults}
				<ChartPanel
					label={formatLogAttributeTarget(chart.target)}
					lines={chart}
					endpoints={LOG_CHART_ENDPOINTS}
					body={{ attribute: { scope: chart.target.scope, key: chart.target.key } }}
					selection={chartSelection}
					{timezone}
					onChange={(lines) => chart && setChart({ ...chart, ...lines })}
					onClose={closeChart}
					onRangeSelect={handleVolumeRangeSelect}
				/>
			{/if}
			<div
				bind:this={scrollEl}
				bind:clientHeight={viewportH}
				onscroll={() => (scrollTop = scrollEl?.scrollTop ?? 0)}
				class="overflow-auto [&_[data-slot=table-container]]:overflow-visible"
			>
				<Table.Root>
					{#if loading}
						<Table.Body>
							<Table.Row>
								<Table.Cell colspan={5} class="h-48">
									<div class="flex h-full items-center justify-center">
										<LoadingCircle size="xlg" />
									</div>
								</Table.Cell>
							</Table.Row>
						</Table.Body>
					{:else if error}
						<Table.Body>
							<Table.Row>
								<Table.Cell colspan={5} class="h-24 text-center text-red-500">
									{error}
								</Table.Cell>
							</Table.Row>
						</Table.Body>
					{:else if logs.length === 0}
						<Table.Body>
							<TableEmptyState colspan={5} message="No logs found." />
						</Table.Body>
					{:else}
						<!-- Solid thead bg so rows can't show through the translucent th
					     bg-muted/60 while it is sticky; the th keeps its own darker
					     background on top, same look as the endpoints table. -->
						<Table.Header class="sticky top-0 z-10 bg-background">
							<Table.Row>
								<TracewayTableHeader
									label="Timestamp"
									tooltip="Log event time"
									class="w-[180px]"
									sortField="timestamp"
									currentSortField={sortField}
									{sortDirection}
									onSort={handleSort}
								/>
								<TracewayTableHeader
									label="Level"
									tooltip="Log severity"
									class="w-[90px]"
									sortField="severity_number"
									currentSortField={sortField}
									{sortDirection}
									onSort={handleSort}
								/>
								<TracewayTableHeader label="Message" tooltip="Log body (click row to expand)" />
								<TracewayTableHeader
									label="Service"
									tooltip="Emitting service"
									class="w-[160px]"
									sortField="service_name"
									currentSortField={sortField}
									{sortDirection}
									onSort={handleSort}
								/>
								<TracewayTableHeader
									label="Trace"
									tooltip="Linked trace ID (if present)"
									class="w-[110px]"
								/>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#if topPad > 0}
								<tr aria-hidden="true" style="height: {topPad}px"></tr>
							{/if}
							{#each visibleLogs as log (log.id)}
								<Table.Row
									data-log-id={log.id}
									class="group cursor-pointer"
									onclick={() => toggleExpanded(log.id)}
								>
									<Table.Cell class="text-muted-foreground tabular-nums">
										{formatDateTime(log.timestamp, { timezone })}
									</Table.Cell>
									<Table.Cell>
										<SeverityBadge
											severityText={log.severityText}
											severityNumber={log.severityNumber}
										/>
									</Table.Cell>
									<Table.Cell class="max-w-[600px] truncate font-mono text-sm">
										<LogMessage body={log.body} attributes={log.logAttributes} />
									</Table.Cell>
									<Table.Cell class="truncate text-muted-foreground">
										{log.serviceName || '—'}
									</Table.Cell>
									<Table.Cell class="font-mono text-xs">
										{#if log.traceId}
											<span class="text-muted-foreground" title={log.traceId}>
												{log.traceId.slice(0, 8)}…
											</span>
										{:else}
											<span class="text-muted-foreground">—</span>
										{/if}
									</Table.Cell>
								</Table.Row>
								{#if expandedId === log.id}
									<ExpandedLogRow
										{log}
										colspan={5}
										filterOperatorFor={attributeFilterOperator}
										onFilterOperator={setAttributeFilterOperator}
										onChart={openChart}
										{isMetaFilterActive}
										onMetaFilterToggle={toggleMetaFilter}
										onHeightChange={setExpandedHeight}
										bind:showAllAttributes
									/>
								{/if}
							{/each}
							{#if bottomPad > 0}
								<tr aria-hidden="true" style="height: {bottomPad}px"></tr>
							{/if}
							<Table.Row>
								<Table.Cell colspan={5}>
									<div class="flex items-center justify-center gap-3 py-1">
										{#if logs.length < displayTotal}
											<Button variant="outline" size="sm" onclick={loadMore} disabled={loadingMore}>
												{loadingMore ? 'Loading…' : `Load ${pageSize} more`}
											</Button>
										{/if}
										{#if loadMoreError}
											<span class="text-xs text-red-500">{loadMoreError}</span>
										{/if}
										<span class="text-xs text-muted-foreground">
											Showing {logs.length.toLocaleString()} of {displayTotal.toLocaleString()} logs
										</span>
									</div>
								</Table.Cell>
							</Table.Row>
						</Table.Body>
					{/if}
				</Table.Root>
			</div>
		</TableContainer>
	{/snippet}
</div>
