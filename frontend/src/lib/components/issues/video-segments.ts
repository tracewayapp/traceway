/**
 * The Flutter SDK records the screen as MP4 clips: one for an issue, or a segment every few seconds while a session
 * streams. The player plays a session's segments back to back as one recording. Time in which nothing was recorded
 * (the app in the background, streaming off for a while) is skipped: playback goes straight from the last frame of
 * one segment to the first frame of the next, and the gap is marked where they meet.
 */

export interface VideoSegmentInput {
	durationSeconds: number;
	/** ISO time of the segment's first frame, when known. */
	startedAt?: string;
	/** ISO time of its last frame, when known. */
	endedAt?: string;
}

export interface PlacedSegment {
	/** Position of the segment in the input list. */
	index: number;
	/** Where the segment starts on the player's timeline, in ms. */
	offsetMs: number;
	durationMs: number;
	/** The wall-clock span the segment shows, in epoch ms; null when the segment does not say. */
	wallStartMs: number | null;
	wallEndMs: number | null;
	/** Wall-clock time nothing was recorded in, between the previous segment and this one. */
	gapBeforeMs: number;
}

export interface SegmentTimeline {
	segments: PlacedSegment[];
	totalMs: number;
	/** Every segment has its wall-clock span, so a player time maps to a moment and back. */
	timed: boolean;
}

/** Gaps shorter than this are frame timing, not time without recording. */
export const MIN_GAP_MS = 1000;

function parseTime(value: string | undefined): number | null {
	if (!value) return null;
	const ms = Date.parse(value);
	return Number.isFinite(ms) ? ms : null;
}

function validDuration(ms: number | undefined): ms is number {
	return ms !== undefined && Number.isFinite(ms) && ms > 0;
}

/**
 * Lays segments out on one timeline. A segment's duration is the one its video reports once loaded
 * (`measuredMs[index]`), else the one the SDK declared. Timed segments play in the order they were recorded.
 */
export function placeSegments(
	inputs: VideoSegmentInput[],
	measuredMs: Record<number, number> = {}
): SegmentTimeline {
	const pieces = inputs.map((input, index) => {
		const declared = input.durationSeconds * 1000;
		const durationMs = validDuration(measuredMs[index])
			? measuredMs[index]
			: validDuration(declared)
				? declared
				: 0;
		const wallStartMs = parseTime(input.startedAt);
		const endedAt = parseTime(input.endedAt);
		// The SDK stamps the last frame, which is shown for one frame interval more: the span is at least as long
		// as the video. A longer span means frames were skipped, and the video runs faster than the clock.
		const wallEndMs =
			wallStartMs === null ? null : Math.max(endedAt ?? wallStartMs, wallStartMs + durationMs);
		return { index, durationMs, wallStartMs, wallEndMs };
	});
	const timed = pieces.length > 0 && pieces.every((piece) => piece.wallStartMs !== null);
	if (timed) pieces.sort((a, b) => a.wallStartMs! - b.wallStartMs! || a.index - b.index);

	let offsetMs = 0;
	let previousEnd: number | null = null;
	const segments = pieces.map((piece): PlacedSegment => {
		const gapBeforeMs =
			timed && previousEnd !== null ? Math.max(0, piece.wallStartMs! - previousEnd) : 0;
		const segment = { ...piece, offsetMs, gapBeforeMs };
		offsetMs += piece.durationMs;
		if (piece.wallEndMs !== null) previousEnd = Math.max(previousEnd ?? 0, piece.wallEndMs);
		return segment;
	});
	return { segments, totalMs: offsetMs, timed };
}

/** The segment showing a player time, as its position in `timeline.segments`, and the time within it. */
export function locate(
	timeline: SegmentTimeline,
	playerMs: number
): { position: number; localMs: number } {
	const { segments, totalMs } = timeline;
	if (segments.length === 0) return { position: 0, localMs: 0 };
	const ms = Math.min(Math.max(0, playerMs), totalMs);
	let position = -1;
	for (let i = 0; i < segments.length; i++) {
		// An empty segment is never shown.
		if (segments[i].durationMs > 0 && segments[i].offsetMs <= ms) position = i;
	}
	if (position < 0) position = 0;
	const segment = segments[position];
	return { position, localMs: Math.min(Math.max(0, ms - segment.offsetMs), segment.durationMs) };
}

/**
 * The player time that shows a wall-clock moment. A moment nothing was recorded at shows the next recorded frame:
 * before the first segment its start, in a gap the start of the segment after it, after the last the end.
 */
export function playerTimeAt(timeline: SegmentTimeline, wallMs: number): number {
	if (!timeline.timed) return Math.min(Math.max(0, wallMs), timeline.totalMs);
	for (const segment of timeline.segments) {
		const start = segment.wallStartMs!;
		const end = segment.wallEndMs!;
		if (wallMs < start) return segment.offsetMs;
		if (wallMs <= end) {
			const fraction = end > start ? (wallMs - start) / (end - start) : 0;
			return segment.offsetMs + fraction * segment.durationMs;
		}
	}
	return timeline.totalMs;
}

/** The wall-clock moment a player time shows; for an untimed timeline the player time itself. */
export function wallTimeAt(timeline: SegmentTimeline, playerMs: number): number {
	if (!timeline.timed || timeline.segments.length === 0) return playerMs;
	const { position, localMs } = locate(timeline, playerMs);
	const segment = timeline.segments[position];
	const start = segment.wallStartMs!;
	const span = segment.wallEndMs! - start;
	return segment.durationMs > 0 ? start + (localMs / segment.durationMs) * span : start;
}

/**
 * A paused video that was never seeked can stay black until it plays (Safari, Firefox, some Chrome builds); a seek
 * just past 0 makes the browser decode and show the first frame.
 */
export const FIRST_FRAME_SECONDS = 0.001;

/**
 * Where a segment that just loaded starts, in seconds: at its end, at a time into it, or on its first frame when it
 * shows paused; null leaves it where it is. A fragmented or damaged MP4 reports a duration that is not finite, and
 * a media element throws on such a time, so the end is then the one the SDK declared.
 */
export function loadedSeekSeconds(
	next: { localMs: number; play: boolean; toEnd: boolean },
	videoDuration: number,
	declaredMs: number
): number | null {
	const duration =
		Number.isFinite(videoDuration) && videoDuration > 0
			? videoDuration
			: validDuration(declaredMs)
				? declaredMs / 1000
				: null;
	if (next.toEnd && duration !== null) return duration;
	if (Number.isFinite(next.localMs) && next.localMs > 0) {
		return duration === null ? next.localMs / 1000 : Math.min(next.localMs / 1000, duration);
	}
	return next.play ? null : FIRST_FRAME_SECONDS;
}
