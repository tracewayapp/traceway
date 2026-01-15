import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { CalendarDate } from '@internationalized/date';
import { getNow, parseISO } from './formatters';

export const presetMinutes: Record<string, number> = {
	'30m': 30,
	'60m': 60,
	'3h': 180,
	'6h': 360,
	'12h': 720,
	'24h': 1440,
	'3d': 4320,
	'7d': 10080,
	'1M': 43200,
	'3M': 129600
};

export function getTimeRangeFromPreset(presetValue: string, timezone: string): { from: Date; to: Date } {
	const minutes = presetMinutes[presetValue] || 360;
	const now = getNow(timezone);
	const from = now.minus({ minutes });
	return { from: from.toJSDate(), to: now.toJSDate() };
}

export function dateToCalendarDate(date: Date): CalendarDate {
	return new CalendarDate(date.getFullYear(), date.getMonth() + 1, date.getDate());
}

export function dateToTimeString(date: Date): string {
	return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

export type TimeRangeParams = {
	preset: string | null;
	from: Date | null;
	to: Date | null;
};

export function parseTimeRangeFromUrl(timezone: string, defaultPreset = '6h'): TimeRangeParams {
	if (!browser) return { preset: defaultPreset, from: null, to: null };

	const params = new URLSearchParams(window.location.search);
	const presetParam = params.get('preset');
	const fromParam = params.get('from');
	const toParam = params.get('to');

	// If preset is specified, use it
	if (presetParam && presetMinutes[presetParam]) {
		return { preset: presetParam, from: null, to: null };
	}

	// If custom from/to specified
	if (fromParam && toParam) {
		const fromDt = parseISO(fromParam, timezone);
		const toDt = parseISO(toParam, timezone);
		if (fromDt.isValid && toDt.isValid) {
			return { preset: null, from: fromDt.toJSDate(), to: toDt.toJSDate() };
		}
	}

	// Default to preset
	return { preset: defaultPreset, from: null, to: null };
}

export function getResolvedTimeRange(
	params: TimeRangeParams,
	timezone: string
): { from: Date; to: Date } {
	if (params.preset) {
		return getTimeRangeFromPreset(params.preset, timezone);
	}
	return { from: params.from!, to: params.to! };
}

export type UpdateUrlOptions = {
	pushToHistory?: boolean;
};

export function updateUrl(
	params: Record<string, string | null | undefined>,
	options: UpdateUrlOptions = {}
): void {
	if (!browser) return;

	const { pushToHistory = true } = options;
	const urlParams = new URLSearchParams();

	for (const [key, value] of Object.entries(params)) {
		if (value != null && value !== '') {
			urlParams.set(key, value);
		}
	}

	const newUrl = `${window.location.pathname}?${urlParams.toString()}`;

	goto(newUrl, {
		replaceState: !pushToHistory,
		noScroll: true,
		keepFocus: true
	});
}
