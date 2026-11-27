import type { eventWithTime } from '@rrweb/types';
import type { FlutterVideoEvent, SessionRecording } from '$lib/types/exceptions';

type JsonObject = Record<string, unknown>;

function isObject(value: unknown): value is JsonObject {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * The objects of a list from a recording. A recording holds what an SDK uploaded, so anything else in the list, or
 * a value that is no list, is left out rather than read as a row.
 */
export function objectRows<T>(rows: unknown): T[] {
	return Array.isArray(rows) ? (rows.filter(isObject) as T[]) : [];
}

/** A recording as the API returned it, with lists that hold only objects; null when there is none. */
export function cleanRecording(raw: unknown): SessionRecording | null {
	if (!isObject(raw)) return null;
	return {
		events: objectRows(raw.events),
		logs: objectRows(raw.logs),
		actions: objectRows(raw.actions),
		startedAt: typeof raw.startedAt === 'string' ? raw.startedAt : undefined,
		endedAt: typeof raw.endedAt === 'string' ? raw.endedAt : undefined,
		truncated: raw.truncated === true
	};
}

// rrweb's event types run from DomContentLoaded (0) to Plugin (6).
const FULL_SNAPSHOT = 2;
const MAX_EVENT_TYPE = 6;
// The range of a Date.
const MAX_TIME_MS = 8.64e15;

/**
 * Whether rrweb can take an event. Its Replayer sorts the events by timestamp, reads the data of every incremental
 * snapshot and Meta event while it starts, and rebuilds the page from a full snapshot's node; any of those missing
 * throws and leaves the whole replay empty.
 */
function isRrwebEvent(event: JsonObject): boolean {
	const { type, timestamp, data } = event;
	if (typeof type !== 'number' || !Number.isInteger(type) || type < 0 || type > MAX_EVENT_TYPE)
		return false;
	if (typeof timestamp !== 'number' || !(Math.abs(timestamp) <= MAX_TIME_MS)) return false;
	if (type >= FULL_SNAPSHOT && !isObject(data)) return false;
	return type !== FULL_SNAPSHOT || isObject((data as JsonObject).node);
}

function isVideoEvent(event: JsonObject): boolean {
	return event.type === 'flutter_video' && typeof event.data === 'string' && event.data !== '';
}

export interface ReplayEvents {
	/** What the recording is, by its first event that can be shown. */
	kind: 'video' | 'rrweb' | 'none';
	video: FlutterVideoEvent[];
	/** In time order: rrweb plays its events sorted, and counts time from the earliest. */
	rrweb: eventWithTime[];
	/** rrweb can start on it: it needs two events, and a full snapshot to show anything. */
	playable: boolean;
}

/**
 * The events of a recording the player can show. A session is recorded one way, but an app that restarts in it
 * after its capture mode changed adds segments of the other kind; those are left out.
 */
export function replayEvents(events: unknown): ReplayEvents {
	const video: FlutterVideoEvent[] = [];
	const rrweb: eventWithTime[] = [];
	let kind: ReplayEvents['kind'] = 'none';
	for (const event of objectRows<JsonObject>(events)) {
		if (isVideoEvent(event)) {
			video.push(event as unknown as FlutterVideoEvent);
			if (kind === 'none') kind = 'video';
		} else if (isRrwebEvent(event)) {
			rrweb.push(event as unknown as eventWithTime);
			if (kind === 'none') kind = 'rrweb';
		}
	}
	if (kind === 'video') return { kind, video, rrweb: [], playable: false };
	rrweb.sort((a, b) => a.timestamp - b.timestamp);
	const playable = rrweb.length >= 2 && rrweb.some((event) => event.type === FULL_SNAPSHOT);
	return { kind, video: [], rrweb, playable };
}

/**
 * The moment a recording's times count from: its startedAt, else its earliest rrweb event, where the player counts
 * from without one. Null when it says neither, like a Flutter clip from an SDK that did not stamp it.
 */
export function recordingOrigin(recording: SessionRecording | null | undefined): string | null {
	if (recording?.startedAt && Number.isFinite(Date.parse(recording.startedAt))) {
		return recording.startedAt;
	}
	let earliest = Infinity;
	for (const event of objectRows<JsonObject>(recording?.events)) {
		if (isRrwebEvent(event)) earliest = Math.min(earliest, event.timestamp as number);
	}
	return Number.isFinite(earliest) ? new Date(earliest).toISOString() : null;
}
