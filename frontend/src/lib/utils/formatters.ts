import { DateTime } from 'luxon';
import { getTimezone } from '$lib/state/timezone.svelte';

export function formatDuration(nanoseconds: number): string {
	const ms = nanoseconds / 1_000_000;
	if (ms < 1) {
		return `${(nanoseconds / 1000).toFixed(0)}µs`;
	} else if (ms < 1000) {
		return `${ms.toFixed(0)}ms`;
	} else {
		return `${(ms / 1000).toFixed(1)}s`;
	}
}

export function formatDurationMs(ms: number): string {
	if (ms < 1) {
		return `${(ms * 1000).toFixed(0)} µs`;
	} else if (ms < 1000) {
		return `${ms.toFixed(0)} ms`;
	} else {
		return `${(ms / 1000).toFixed(2)} s`;
	}
}

export function getStatusColor(statusCode: number): string {
	if (statusCode >= 200 && statusCode < 300) return 'text-green-500';
	if (statusCode >= 300 && statusCode < 400) return 'text-blue-500';
	if (statusCode >= 400 && statusCode < 500) return 'text-yellow-500';
	return 'text-red-500';
}

export function truncateStackTrace(stackTrace: string, maxLength = 70): string {
	const firstLine = stackTrace.split('\n')[0];
	if (firstLine.length > maxLength) {
		return firstLine.slice(0, maxLength) + '...';
	}
	return firstLine;
}

export function formatRelativeTime(dateStr: string, timezone?: string): string {
	const tz = timezone ?? getTimezone();
	const dt = DateTime.fromISO(dateStr, { zone: 'utc' }).setZone(tz);
	const now = DateTime.now().setZone(tz);
	const diffMs = now.toMillis() - dt.toMillis();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m`;
	if (diffHours < 24) return `${diffHours}h`;
	return `${diffDays}d`;
}

export type DateTimeFormat = 'full' | 'short' | 'date' | 'time' | 'datetime' | 'iso';

export function formatDateTime(
	dateInput: string | Date | number,
	options: {
		timezone?: string;
		format?: DateTimeFormat;
	} = {}
): string {
	const tz = options.timezone ?? getTimezone();
	const isoStr =
		typeof dateInput === 'string'
			? dateInput
			: dateInput instanceof Date
				? dateInput.toISOString()
				: new Date(dateInput).toISOString();

	const dt = DateTime.fromISO(isoStr, { zone: 'utc' }).setZone(tz);

	if (!dt.isValid) {
		return 'Invalid date';
	}

	switch (options.format) {
		case 'full':
			return dt.toLocaleString(DateTime.DATETIME_FULL);
		case 'short':
			return dt.toLocaleString(DateTime.DATETIME_SHORT);
		case 'date':
			return dt.toLocaleString(DateTime.DATE_MED);
		case 'time':
			return dt.toLocaleString(DateTime.TIME_SIMPLE);
		case 'iso':
			return dt.toISO() ?? '';
		case 'datetime':
		default:
			return dt.toLocaleString(DateTime.DATETIME_MED);
	}
}

export function getNow(timezone?: string): DateTime {
	const tz = timezone ?? getTimezone();
	return DateTime.now().setZone(tz);
}



export function luxonToCalendarDateTime(dt: DateTime): {
	year: number;
	month: number;
	day: number;
	hour: number;
	minute: number;
	second: number;
} {
	return {
		year: dt.year,
		month: dt.month,
		day: dt.day,
		hour: dt.hour,
		minute: dt.minute,
		second: dt.second
	};
}

export function calendarDateTimeToLuxon(
	calDt: {
		year: number;
		month: number;
		day: number;
		hour?: number;
		minute?: number;
		second?: number;
	},
	timezone?: string
): DateTime {
	const tz = timezone ?? getTimezone();
	return DateTime.fromObject(
		{
			year: calDt.year,
			month: calDt.month,
			day: calDt.day,
			hour: calDt.hour ?? 0,
			minute: calDt.minute ?? 0,
			second: calDt.second ?? 0
		},
		{ zone: tz }
	);
}

export function toUTCISO(dt: DateTime): string {
	return dt.toUTC().toISO() ?? '';
}

export function parseISO(dateStr: string, timezone?: string): DateTime {
	const tz = timezone ?? getTimezone();
	return DateTime.fromISO(dateStr, { zone: 'utc' }).setZone(tz);
}
