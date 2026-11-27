import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/svelte';

type FakePlayer = {
	events: unknown[];
	goto: ReturnType<typeof vi.fn>;
	getReplayer: () => { getCurrentTime: () => number };
	$set: ReturnType<typeof vi.fn>;
	triggerResize: ReturnType<typeof vi.fn>;
	$destroy: ReturnType<typeof vi.fn>;
};

const state = vi.hoisted(() => ({
	framework: 'react',
	throwOnCreate: false,
	players: [] as FakePlayer[]
}));

vi.mock('rrweb-player', () => ({
	default: class {
		constructor(options: { props: { events: unknown[] } }) {
			if (state.throwOnCreate) throw new Error('Replayer need at least 2 events.');
			const player: FakePlayer = {
				events: options.props.events,
				goto: vi.fn(),
				getReplayer: () => ({ getCurrentTime: () => 0 }),
				$set: vi.fn(),
				triggerResize: vi.fn(),
				$destroy: vi.fn()
			};
			state.players.push(player);
			return player;
		}
	}
}));
vi.mock('$lib/state/projects.svelte', () => ({
	projectsState: {
		get currentProject() {
			return { framework: state.framework };
		}
	}
}));

import SessionReplay from './session-replay.svelte';

const meta = (timestamp: number) => ({
	type: 4,
	timestamp,
	data: { href: 'https://shop.test/cart', width: 1280, height: 720 }
});
const full = (timestamp: number) => ({
	type: 2,
	timestamp,
	data: { node: { type: 0, id: 1, childNodes: [] }, initialOffset: { top: 0, left: 0 } }
});
const click = (timestamp: number) => ({
	type: 3,
	timestamp,
	data: { source: 2, type: 2, id: 1, x: 10, y: 10 }
});
const video = (durationSeconds = 1) => ({
	type: 'flutter_video',
	data: 'AAAA',
	format: 'mp4',
	fps: 10,
	durationSeconds
});

const errors: unknown[] = [];
const onError = (event: ErrorEvent) => errors.push(event.error);
const onRejection = (event: PromiseRejectionEvent) => errors.push(event.reason);

beforeEach(() => {
	state.framework = 'react';
	state.throwOnCreate = false;
	state.players.length = 0;
	errors.length = 0;
	vi.stubGlobal(
		'ResizeObserver',
		class {
			observe() {}
			disconnect() {}
		}
	);
	window.addEventListener('error', onError);
	window.addEventListener('unhandledrejection', onRejection);
});

afterEach(() => {
	cleanup();
	window.removeEventListener('error', onError);
	window.removeEventListener('unhandledrejection', onRejection);
	vi.unstubAllGlobals();
});

/** The player loads rrweb-player in onMount and seeks to the end in a timeout. */
async function settle() {
	await new Promise((resolve) => setTimeout(resolve, 0));
	await new Promise((resolve) => setTimeout(resolve, 0));
}

describe('session replay with malformed recordings (TW-2)', () => {
	it('renders a browser recording that holds a null event', async () => {
		const events = [null, meta(1000), full(1000)] as never;
		expect(() => render(SessionReplay, { props: { events } })).not.toThrow();
		await waitFor(() => expect(state.players).toHaveLength(1));
		expect(state.players[0].events).toEqual([meta(1000), full(1000)]);
		expect(screen.getByText('https://shop.test/cart')).toBeTruthy();
	});

	it('renders a recording that is not a list of events', () => {
		expect(() => render(SessionReplay, { props: { events: 'abc' as never } })).not.toThrow();
		expect(state.players).toHaveLength(0);
	});

	it('passes the player only the events rrweb can replay', async () => {
		state.framework = 'android';
		const events = [
			meta(1000),
			full(1000),
			{ type: 3, timestamp: 1500 },
			{ type: 4, timestamp: 1600 },
			{ type: 2, timestamp: 1700, data: {} },
			{ type: 3, timestamp: Number.NaN, data: { source: 2 } },
			{ type: '3', timestamp: 1800, data: {} },
			video()
		] as never;
		render(SessionReplay, { props: { events } });
		await waitFor(() => expect(state.players).toHaveLength(1));
		expect(state.players[0].events).toEqual([meta(1000), full(1000)]);
	});

	it('says a recording with too little to play cannot be played', async () => {
		state.framework = 'android';
		render(SessionReplay, { props: { events: [meta(1000)] as never } });
		await settle();
		expect(state.players).toHaveLength(0);
		expect(screen.getByText(/cannot be played/i)).toBeTruthy();
	});

	it('says so when the player fails to start, without an unhandled error', async () => {
		state.throwOnCreate = true;
		render(SessionReplay, { props: { events: [meta(1000), full(1000)] as never } });
		await waitFor(() => expect(screen.getByText(/cannot be played/i)).toBeTruthy());
		await settle();
		expect(errors).toEqual([]);
	});
});

describe('session replay time origin (TW-4)', () => {
	it('counts from the earliest event when segments arrive out of time order', async () => {
		vi.useFakeTimers({ toFake: ['setTimeout'] });
		try {
			const a = [meta(10_000), full(10_000), click(12_000)];
			const b = [meta(0), full(0), click(2_000)];
			const { component } = render(SessionReplay, {
				props: { events: [...a, ...b] as never, startedAt: new Date(0).toISOString() }
			});
			await vi.waitFor(() => expect(state.players).toHaveLength(1));
			vi.runAllTimers();
			const player = state.players[0];
			expect(player.goto).toHaveBeenCalledWith(12_000, true);
			component.seek(500);
			expect(player.goto).toHaveBeenLastCalledWith(501, false);
		} finally {
			vi.useRealTimers();
		}
	});
});

describe('flutter video with a duration that is not finite (TW-6)', () => {
	function setDuration(element: HTMLVideoElement, duration: number) {
		Object.defineProperty(element, 'duration', { configurable: true, value: duration });
	}

	it('opens a single clip without throwing', () => {
		for (const duration of [Infinity, Number.NaN]) {
			const { container, unmount } = render(SessionReplay, {
				props: { events: [video()] as never }
			});
			const element = container.querySelector('video')!;
			setDuration(element, duration);
			element.dispatchEvent(new Event('loadedmetadata'));
			expect(errors).toEqual([]);
			// The clip opens at the end the SDK declared.
			expect(element.currentTime).toBe(1);
			unmount();
		}
	});

	it('seeks into another segment without throwing', () => {
		const { container, component } = render(SessionReplay, {
			props: { events: [video(1), video(1)] as never }
		});
		const element = container.querySelector('video')!;
		element.dispatchEvent(new Event('loadedmetadata'));
		component.seek(1500);
		setDuration(element, Number.NaN);
		element.dispatchEvent(new Event('loadedmetadata'));
		expect(errors).toEqual([]);
		expect(element.currentTime).toBe(0.5);
	});
});
