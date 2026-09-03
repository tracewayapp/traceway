import { CalendarDate } from '@internationalized/date';
import { getTimezone } from '$lib/state/timezone.svelte';
import { calendarDateTimeToLuxon, toUTCISO } from './formatters';
import {
	dateToCalendarDate,
	dateToTimeString,
	getResolvedTimeRange,
	getTimeRangeFromPreset,
	parseTimeRangeFromUrl
} from './url-params';

export class TimeRangeState {
	preset = $state<string | null>(null);
	fromDate = $state<CalendarDate>(new CalendarDate(1970, 1, 1));
	toDate = $state<CalendarDate>(new CalendarDate(1970, 1, 1));
	fromTime = $state('00:00');
	toTime = $state('00:00');

	readonly defaultPreset: string;

	constructor(defaultPreset: string) {
		this.defaultPreset = defaultPreset;
		this.readUrl();
	}

	readUrl() {
		const timezone = getTimezone();
		const params = parseTimeRangeFromUrl(timezone, this.defaultPreset);
		const range = getResolvedTimeRange(params, timezone);
		this.preset = params.preset;
		this.setRange(range.from, range.to, timezone);
	}

	resolvePreset() {
		if (!this.preset) return;
		const timezone = getTimezone();
		const range = getTimeRangeFromPreset(this.preset, timezone);
		this.setRange(range.from, range.to, timezone);
	}

	apply(
		from: { date: CalendarDate; time: string },
		to: { date: CalendarDate; time: string },
		preset: string | null
	) {
		this.fromDate = from.date;
		this.toDate = to.date;
		this.fromTime = from.time;
		this.toTime = to.time;
		this.preset = preset;
	}

	fromUTC(): string {
		return toUTCISO(this.toLuxon(this.fromDate, this.fromTime));
	}

	toUTC(): string {
		return toUTCISO(this.toLuxon(this.toDate, this.toTime).endOf('minute'));
	}

	urlParams(): Record<string, string | null> {
		if (this.preset) return { preset: this.preset, from: null, to: null };
		return { preset: null, from: this.fromUTC(), to: this.toUTC() };
	}

	private setRange(from: Date, to: Date, timezone: string) {
		this.fromDate = dateToCalendarDate(from, timezone);
		this.toDate = dateToCalendarDate(to, timezone);
		this.fromTime = dateToTimeString(from, timezone);
		this.toTime = dateToTimeString(to, timezone);
	}

	private toLuxon(date: CalendarDate, time: string) {
		const [hour, minute] = time.split(':').map(Number);
		return calendarDateTimeToLuxon(
			{ year: date.year, month: date.month, day: date.day, hour, minute },
			getTimezone()
		);
	}
}
