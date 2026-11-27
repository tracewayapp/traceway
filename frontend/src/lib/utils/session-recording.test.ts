import { describe, expect, it } from 'vitest';
import { cleanRecording, objectRows, recordingOrigin, replayEvents } from './session-recording';

const meta = (timestamp: number) => ({
	type: 4,
	timestamp,
	data: { href: '/', width: 1, height: 1 }
});
const full = (timestamp: number) => ({ type: 2, timestamp, data: { node: { type: 0 } } });
const move = (timestamp: number) => ({ type: 3, timestamp, data: { source: 1 } });
const video = { type: 'flutter_video', data: 'AAAA', format: 'mp4', fps: 10, durationSeconds: 1 };

describe('session recording clean-up', () => {
	it('keeps the objects of a list and nothing of a value that is no list', () => {
		expect(objectRows([null, 1, 'x', [2], { a: 1 }])).toEqual([{ a: 1 }]);
		for (const value of [undefined, null, {}, 'abc', 5]) expect(objectRows(value)).toEqual([]);
	});

	it('reads a recording as the API returned it', () => {
		expect(cleanRecording(null)).toBeNull();
		expect(cleanRecording('abc')).toBeNull();
		expect(
			cleanRecording({
				events: 'abc',
				logs: { level: 'error' },
				actions: [null, { type: 'custom' }],
				startedAt: 5,
				endedAt: '2026-11-27T10:00:00Z',
				truncated: true
			})
		).toEqual({
			events: [],
			logs: [],
			actions: [{ type: 'custom' }],
			startedAt: undefined,
			endedAt: '2026-11-27T10:00:00Z',
			truncated: true
		});
	});

	it('keeps the rrweb events the player can take, in time order', () => {
		const replay = replayEvents([
			move(30),
			null,
			meta(10),
			full(10),
			{ type: 3, timestamp: 20 },
			{ type: 4, timestamp: 20 },
			{ type: 2, timestamp: 20, data: {} },
			{ type: 3, timestamp: '20', data: {} },
			{ type: 3, timestamp: Infinity, data: {} },
			{ type: 9, timestamp: 20, data: {} },
			video
		]);
		expect(replay).toEqual({
			kind: 'rrweb',
			video: [],
			rrweb: [meta(10), full(10), move(30)],
			playable: true
		});
	});

	it('needs two events and a full snapshot to play rrweb', () => {
		expect(replayEvents([meta(1)]).playable).toBe(false);
		expect(replayEvents([meta(1), move(2)]).playable).toBe(false);
		expect(replayEvents([full(1), move(2)]).playable).toBe(true);
		expect(replayEvents('abc')).toEqual({ kind: 'none', video: [], rrweb: [], playable: false });
	});

	it('plays a video recording without the rrweb events of another mode', () => {
		expect(replayEvents([null, video, meta(1), full(1), { ...video, data: 5 }, video])).toEqual({
			kind: 'video',
			video: [video, video],
			rrweb: [],
			playable: false
		});
	});

	it('plays the video when the rrweb events before it cannot play', () => {
		expect(replayEvents([meta(1), video])).toEqual({
			kind: 'video',
			video: [video],
			rrweb: [],
			playable: false
		});
		expect(replayEvents([full(1), move(2), video]).kind).toBe('rrweb');
	});

	it('counts a recording from its start, else its earliest event', () => {
		const startedAt = '2026-11-27T10:00:00.000Z';
		const t = Date.parse(startedAt);
		expect(recordingOrigin({ startedAt, events: [meta(t + 5)] as never })).toBe(startedAt);
		expect(recordingOrigin({ events: [move(t + 9), null, meta(t + 5)] as never })).toBe(
			new Date(t + 5).toISOString()
		);
		expect(recordingOrigin({ startedAt: 'later', events: [meta(t)] as never })).toBe(startedAt);
		expect(recordingOrigin({ events: [video] as never })).toBeNull();
		expect(recordingOrigin(null)).toBeNull();
	});
});
