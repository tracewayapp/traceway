import { describe, expect, it } from 'vitest';
import { locate, placeSegments, playerTimeAt, wallTimeAt } from './video-segments';

const at = (seconds: number) => new Date(Date.UTC(2026, 10, 18, 10, 0, seconds)).toISOString();
const ms = (seconds: number) => Date.UTC(2026, 10, 18, 10, 0, seconds);

describe('video segments', () => {
	it('plays a single clip as it is', () => {
		const timeline = placeSegments([{ durationSeconds: 10 }]);
		expect(timeline.totalMs).toBe(10_000);
		expect(timeline.timed).toBe(false);
		expect(locate(timeline, 4_000)).toEqual({ position: 0, localMs: 4_000 });
		expect(locate(timeline, 60_000)).toEqual({ position: 0, localMs: 10_000 });
		expect(wallTimeAt(timeline, 4_000)).toBe(4_000);
	});

	it('lays consecutive segments end to end', () => {
		const timeline = placeSegments([
			{ durationSeconds: 8, startedAt: at(0), endedAt: at(8) },
			{ durationSeconds: 8, startedAt: at(8), endedAt: at(16) },
			{ durationSeconds: 4, startedAt: at(16), endedAt: at(20) }
		]);
		expect(timeline.timed).toBe(true);
		expect(timeline.totalMs).toBe(20_000);
		expect(timeline.segments.map((s) => s.offsetMs)).toEqual([0, 8_000, 16_000]);
		expect(timeline.segments.every((s) => s.gapBeforeMs === 0)).toBe(true);
		expect(locate(timeline, 7_999)).toEqual({ position: 0, localMs: 7_999 });
		expect(locate(timeline, 8_000)).toEqual({ position: 1, localMs: 0 });
		expect(locate(timeline, 17_500)).toEqual({ position: 2, localMs: 1_500 });
		expect(locate(timeline, 20_000)).toEqual({ position: 2, localMs: 4_000 });
		expect(locate(timeline, -5)).toEqual({ position: 0, localMs: 0 });
	});

	it('uses the duration a loaded video reports over the declared one', () => {
		const timeline = placeSegments([{ durationSeconds: 8 }, { durationSeconds: 8 }], {
			0: 8_400
		});
		expect(timeline.segments.map((s) => s.offsetMs)).toEqual([0, 8_400]);
		expect(timeline.totalMs).toBe(16_400);
	});

	it('skips the time between segments and maps the session clock across the gap', () => {
		const timeline = placeSegments([
			{ durationSeconds: 8, startedAt: at(0), endedAt: at(8) },
			// The app was in the background for 52 seconds.
			{ durationSeconds: 8, startedAt: at(60), endedAt: at(68) }
		]);
		expect(timeline.totalMs).toBe(16_000);
		expect(timeline.segments[1].gapBeforeMs).toBe(52_000);
		// A moment inside a segment.
		expect(playerTimeAt(timeline, ms(3))).toBe(3_000);
		expect(playerTimeAt(timeline, ms(62))).toBe(10_000);
		// A moment in the gap shows the next recorded frame; before and after the recording its ends.
		expect(playerTimeAt(timeline, ms(30))).toBe(8_000);
		expect(playerTimeAt(timeline, ms(-10))).toBe(0);
		expect(playerTimeAt(timeline, ms(90))).toBe(16_000);
		// And back.
		expect(wallTimeAt(timeline, 3_000)).toBe(ms(3));
		expect(wallTimeAt(timeline, 10_000)).toBe(ms(62));
		expect(wallTimeAt(timeline, 8_000)).toBe(ms(60));
	});

	it('plays timed segments in the order they were recorded', () => {
		const timeline = placeSegments([
			{ durationSeconds: 8, startedAt: at(20), endedAt: at(28) },
			{ durationSeconds: 8, startedAt: at(0), endedAt: at(8) }
		]);
		expect(timeline.segments.map((s) => s.index)).toEqual([1, 0]);
		expect(timeline.segments[1].gapBeforeMs).toBe(12_000);
		expect(playerTimeAt(timeline, ms(21))).toBe(9_000);
	});

	it('stretches a segment whose frames were skipped over the time it covers', () => {
		// 4 seconds of video for 8 seconds of the clock: the device could not keep up.
		const timeline = placeSegments([{ durationSeconds: 4, startedAt: at(0), endedAt: at(8) }]);
		expect(playerTimeAt(timeline, ms(4))).toBe(2_000);
		expect(wallTimeAt(timeline, 2_000)).toBe(ms(4));
	});

	it('treats a segment without a time as untimed, and skips empty segments', () => {
		const timeline = placeSegments([
			{ durationSeconds: 8, startedAt: at(0) },
			{ durationSeconds: 0 },
			{ durationSeconds: 8 }
		]);
		expect(timeline.timed).toBe(false);
		expect(timeline.segments.map((s) => s.offsetMs)).toEqual([0, 8_000, 8_000]);
		expect(locate(timeline, 8_000)).toEqual({ position: 2, localMs: 0 });
		expect(playerTimeAt(timeline, 5_000)).toBe(5_000);
		expect(wallTimeAt(timeline, 5_000)).toBe(5_000);
	});

	it('handles an empty recording', () => {
		const timeline = placeSegments([]);
		expect(timeline).toEqual({ segments: [], totalMs: 0, timed: false });
		expect(locate(timeline, 1_000)).toEqual({ position: 0, localMs: 0 });
		expect(wallTimeAt(timeline, 1_000)).toBe(1_000);
	});
});
