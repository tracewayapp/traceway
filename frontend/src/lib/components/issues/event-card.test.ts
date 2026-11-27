import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';

const state = vi.hoisted(() => ({
	players: [] as Array<{ goto: ReturnType<typeof vi.fn> }>
}));

vi.mock('rrweb-player', () => ({
	default: class {
		constructor() {
			const player = {
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
vi.mock('$lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));
vi.mock('$lib/state/projects.svelte', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/state/projects.svelte')>()),
	projectsState: {
		currentProjectId: 'project-1',
		currentProject: { id: 'project-1', framework: 'react' },
		canWriteCurrentProject: false
	}
}));

import EventCard from '../../../test/event-card.svelte';

const T = Date.parse('2026-11-27T10:00:00Z');
const iso = (ms: number) => new Date(ms).toISOString();

beforeEach(() => {
	state.players.length = 0;
	vi.stubGlobal(
		'ResizeObserver',
		class {
			observe() {}
			disconnect() {}
		}
	);
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

// TW-7: a clip without startedAt (any 1.x clip, or a 2.x upload without it) counts its breadcrumb offsets from its
// first event, the same clock the player seeks on, not from the exception.
it('seeks the clip to an action of a recording that has no startedAt', async () => {
	render(EventCard, {
		occurrence: {
			id: 'e1',
			traceId: '',
			spanId: '',
			traceType: '',
			exceptionHash: 'h1',
			stackTrace: 'Error: boom',
			recordedAt: iso(T + 10_000),
			attributes: {},
			appVersion: '1.0.0',
			serverName: '',
			isMessage: false,
			endpoint: ''
		},
		linkedTrace: null,
		sessionRecording: {
			events: [
				{
					type: 4,
					timestamp: T + 5_000,
					data: { href: 'https://shop.test/', width: 1280, height: 720 }
				},
				{ type: 2, timestamp: T + 5_000, data: { node: { type: 0, id: 1, childNodes: [] } } }
			] as never,
			actions: [
				{ type: 'navigation', timestamp: iso(T + 7_000), action: 'push', from: '/', to: '/cart' }
			]
		}
	});
	await waitFor(() => expect(state.players).toHaveLength(1));
	const row = await screen.findByText('+2.00s');
	await fireEvent.click(row);
	expect(state.players[0].goto).toHaveBeenLastCalledWith(2_001, false);
	expect(screen.getByText(/Times are offsets from the start of the recording/)).toBeTruthy();
});

// Crash safety: an exception's clip holds what its SDK uploaded; the issue page shows what it can of it.
it('renders an issue whose clip is malformed', async () => {
	const occurrence = {
		id: 'e1',
		traceId: '',
		spanId: '',
		traceType: '' as const,
		exceptionHash: 'h1',
		stackTrace: 'Error: boom',
		recordedAt: iso(T),
		attributes: {},
		appVersion: '1.0.0',
		serverName: '',
		isMessage: false,
		endpoint: ''
	};
	for (const sessionRecording of [
		{ events: 'abc', logs: {}, actions: [null, 5] },
		{ events: [null], logs: [null], actions: {} },
		{ events: [{ type: 3, timestamp: T }], logs: null, actions: 'x', startedAt: 5 },
		'abc'
	]) {
		const { unmount } = render(EventCard, {
			occurrence,
			linkedTrace: null,
			sessionRecording: sessionRecording as never
		});
		expect(screen.getByText('Recorded At')).toBeTruthy();
		expect(screen.getByText(/Replay unavailable|cannot be played/)).toBeTruthy();
		unmount();
	}
	expect(state.players).toHaveLength(0);
});
