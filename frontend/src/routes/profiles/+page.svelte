<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api';
	import { formatRelativeTime, toUTCISO, calendarDateTimeToLuxon } from '$lib/utils/formatters';
	import { formatValue, humanizeType } from '$lib/utils/profile-format';
	import { getTimezone } from '$lib/state/timezone.svelte';
	import * as Table from '$lib/components/ui/table';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { TracewayTableHeader } from '$lib/components/ui/traceway-table-header';
	import { TableEmptyState } from '$lib/components/ui/table-empty-state';
	import { PaginationFooter } from '$lib/components/ui/pagination-footer';
	import { TimeRangePicker } from '$lib/components/ui/time-range-picker';
	import { CalendarDate } from '@internationalized/date';
	import { projectsState } from '$lib/state/projects.svelte';
	import { createRowClickHandler } from '$lib/utils/navigation';
	import { resolve } from '$app/paths';
	import PageHeader from '$lib/components/issues/page-header.svelte';
	import {
		getTimeRangeFromPreset,
		dateToCalendarDate,
		dateToTimeString,
		parseTimeRangeFromUrl,
		getResolvedTimeRange,
		updateUrl
	} from '$lib/utils/url-params';
	import {
		getSortState,
		setSortState,
		handleSortClick,
		type SortDirection
	} from '$lib/utils/sort-storage';

	const timezone = $derived(getTimezone());

	type ProfileGroup = {
		serviceName: string;
		type: string;
		unit: string;
		isGauge: boolean;
		profileCount: number;
		sampleCount: number;
		totalValue: number;
		lastSeen: string;
	};

	type SortField = 'profile_count' | 'sample_count' | 'total_value' | 'last_seen';

	let groups = $state<ProfileGroup[]>([]);
	let loading = $state(true);
	let error = $state('');

	let page = $state(1);
	let pageSize = $state(50);
	let total = $state(0);
	let totalPages = $state(0);

	const initialUrlParams = parseTimeRangeFromUrl(timezone);
	const initialRange = getResolvedTimeRange(initialUrlParams, timezone);

	let selectedPreset = $state<string | null>(initialUrlParams.preset);
	let fromDate = $state<CalendarDate>(dateToCalendarDate(initialRange.from, timezone));
	let toDate = $state<CalendarDate>(dateToCalendarDate(initialRange.to, timezone));
	let fromTime = $state(dateToTimeString(initialRange.from, timezone));
	let toTime = $state(dateToTimeString(initialRange.to, timezone));

	function updateTimeRangeUrl(pushToHistory = true) {
		updateUrl(
			selectedPreset
				? { preset: selectedPreset }
				: { from: getFromDateTimeUTC(), to: getToDateTimeUTC() },
			{ pushToHistory }
		);
	}

	function handlePopState() {
		const urlParams = parseTimeRangeFromUrl(timezone);
		const range = getResolvedTimeRange(urlParams, timezone);

		selectedPreset = urlParams.preset;
		fromDate = dateToCalendarDate(range.from, timezone);
		fromTime = dateToTimeString(range.from, timezone);
		toDate = dateToCalendarDate(range.to, timezone);
		toTime = dateToTimeString(range.to, timezone);

		page = 1;
		loadData(false);
	}

	const SORT_STORAGE_KEY = 'profiles';
	const initialSort = getSortState(SORT_STORAGE_KEY, { field: 'total_value', direction: 'desc' });
	let orderBy = $state<SortField>(initialSort.field as SortField);
	let sortDirection = $state<SortDirection>(initialSort.direction);

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
		page = 1;
		loadData(true);
	}

	function formatCount(count: number): string {
		if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}m`;
		if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
		return count.toLocaleString();
	}

	function typeLabel(type: string): string {
		return humanizeType(type);
	}

	function detailHref(group: ProfileGroup): string {
		return `${resolve(`/profiles/${encodeURIComponent(group.serviceName)}`)}?type=${encodeURIComponent(group.type)}`;
	}

	async function loadData(pushToHistory = true) {
		loading = true;
		error = '';

		if (selectedPreset) {
			const range = getTimeRangeFromPreset(selectedPreset, timezone);
			fromDate = dateToCalendarDate(range.from, timezone);
			toDate = dateToCalendarDate(range.to, timezone);
			fromTime = dateToTimeString(range.from, timezone);
			toTime = dateToTimeString(range.to, timezone);
		}

		updateTimeRangeUrl(pushToHistory);

		try {
			const requestBody = {
				fromDate: getFromDateTimeUTC(),
				toDate: getToDateTimeUTC(),
				orderBy,
				sortDirection,
				pagination: { page, pageSize }
			};

			const response = await api.post('/profiles/grouped', requestBody, {
				projectId: projectsState.currentProjectId ?? undefined
			});

			groups = response.data || [];
			total = response.pagination.total;
			totalPages = response.pagination.totalPages;
		} catch (e: any) {
			console.error(e);
			error = e.message || 'Failed to load data';
		} finally {
			loading = false;
		}
	}

	function handlePageChange(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			page = newPage;
			loadData(false);
		}
	}

	function handlePageSizeChange(newPageSize: number) {
		pageSize = newPageSize;
		page = 1;
		loadData(false);
	}

	function handleSort(field: SortField) {
		const newSort = handleSortClick(field, orderBy, sortDirection);
		orderBy = newSort.field as SortField;
		sortDirection = newSort.direction;
		setSortState(SORT_STORAGE_KEY, newSort);
		page = 1;
		loadData(false);
	}

	onMount(() => {
		window.addEventListener('popstate', handlePopState);
		loadData(false);
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('popstate', handlePopState);
		}
	});
</script>

<div class="space-y-4">
	<div class="flex flex-col gap-4 sm:flex-row sm:justify-between">
		<PageHeader title="Profiles" />

		<div class="flex flex-col">
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
			{#if loading}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={6} class="h-48">
							<div class="flex h-full items-center justify-center">
								<LoadingCircle size="xlg" />
							</div>
						</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if error}
				<Table.Body>
					<Table.Row>
						<Table.Cell colspan={6} class="h-24 text-center text-red-500">{error}</Table.Cell>
					</Table.Row>
				</Table.Body>
			{:else if groups.length === 0}
				<Table.Body>
					<TableEmptyState colspan={6} message="No profile data received yet" />
				</Table.Body>
			{:else}
				<Table.Header>
					<Table.Row>
						<TracewayTableHeader label="Service" tooltip="The service that emitted the profile" />
						<TracewayTableHeader
							label="Type"
							tooltip="The profile type (CPU time or heap memory)"
						/>
						<TracewayTableHeader
							label="Profiles"
							tooltip="Number of profiles collected in the range"
							sortField="profile_count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
						<TracewayTableHeader
							label="Samples"
							tooltip="Total samples across all profiles"
							sortField="sample_count"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
						<TracewayTableHeader
							label="Total"
							tooltip="Aggregate value (CPU time or bytes) over the range"
							sortField="total_value"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[120px]"
						/>
						<TracewayTableHeader
							label="Last Seen"
							tooltip="When this service last reported this profile type"
							sortField="last_seen"
							currentSortField={orderBy}
							{sortDirection}
							onSort={(field) => handleSort(field as SortField)}
							class="w-[110px]"
						/>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each groups as group (group.serviceName + '::' + group.type)}
						<Table.Row
							class="cursor-pointer hover:bg-muted/50"
							onclick={createRowClickHandler(detailHref(group), 'preset', 'from', 'to')}
						>
							<Table.Cell class="font-mono text-sm">{group.serviceName}</Table.Cell>
							<Table.Cell class="text-sm">{typeLabel(group.type)}</Table.Cell>
							<Table.Cell class="tabular-nums">{formatCount(group.profileCount)}</Table.Cell>
							<Table.Cell class="tabular-nums">{formatCount(group.sampleCount)}</Table.Cell>
							<Table.Cell class="font-mono text-sm tabular-nums">
								{formatValue(group.unit, group.totalValue)}
							</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">
								{formatRelativeTime(group.lastSeen, timezone)}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			{/if}
		</Table.Root>
	</div>

	<PaginationFooter
		currentPage={page}
		{totalPages}
		{pageSize}
		totalItems={total}
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		{loading}
		itemLabel="profile"
	/>
</div>
