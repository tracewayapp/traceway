import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/svelte';

const { post, get, players } = vi.hoisted(() => ({
	post: vi.fn(),
	get: vi.fn(),
	players: [] as unknown[][]
}));

vi.mock('$lib/api', () => ({ api: { post, get } }));
vi.mock('$app/state', () => ({
	page: {
		params: { sessionId: 'a5100000-0000-4000-8000-000000000000' },
		url: new URL('http://localhost/sessions/a5100000-0000-4000-8000-000000000000')
	}
}));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));
vi.mock('$lib/state/projects.svelte', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/state/projects.svelte')>()),
	projectsState: {
		currentProjectId: 'project-1',
		currentProject: { id: 'project-1', framework: 'react' },
		canWriteCurrentProject: false
	}
}));
vi.mock('rrweb-player', () => ({
	default: class {
		constructor(options: { props: { events: unknown[] } }) {
			players.push(options.props.events);
			return {
				goto: vi.fn(),
				getReplayer: () => ({ getCurrentTime: () => 0 }),
				$set: vi.fn(),
				triggerResize: vi.fn(),
				$destroy: vi.fn()
			};
		}
	}
}));

import SessionPage from '../../../test/session-page.svelte';

const startedAt = '2026-11-27T10:00:00.000Z';
const t = Date.parse(startedAt);
const meta = {
	type: 4,
	timestamp: t,
	data: { href: 'https://shop.test/', width: 1280, height: 720 }
};
const full = { type: 2, timestamp: t, data: { node: { type: 0, id: 1, childNodes: [] } } };
const log = { type: 'log', timestamp: startedAt, level: 'info', message: 'checkout opened' };

function serve(recording: unknown) {
	post.mockResolvedValue({
		session: {
			id: 'a5100000-0000-4000-8000-000000000000',
			startedAt,
			duration: 0,
			clientIP: '',
			appVersion: '1.0.0',
			serverName: ''
		},
		exceptions: []
	});
	get.mockImplementation(async (url: string) =>
		url.includes('/timeline') ? { logs: [], actions: [], truncated: false } : recording
	);
}

beforeEach(() => {
	players.length = 0;
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
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});

// Crash safety: a session's recording holds what its SDK uploaded; the page shows what it can of it.
it('shows a session whose recording holds malformed events and breadcrumbs', async () => {
	serve({ events: [null, meta, 7, full, { type: 3 }], logs: [null, 5, log], actions: { a: 1 } });
	render(SessionPage);
	await waitFor(() => expect(players).toEqual([[meta, full]]));
	expect(await screen.findByText('checkout opened')).toBeTruthy();
});

it('shows a session whose recording is not lists at all', async () => {
	serve({ events: 'abc', logs: 'x', actions: 5, startedAt: 7 });
	render(SessionPage);
	expect(await screen.findByText(/No replay data has been uploaded/)).toBeTruthy();
	expect(players).toEqual([]);
});

it('says when a recording cannot be played or was cut short', async () => {
	serve({ events: [meta, { type: 3, timestamp: t + 1 }], truncated: true });
	render(SessionPage);
	expect(await screen.findByText(/cannot be played/)).toBeTruthy();
	expect(screen.getByText(/the replay stops early/)).toBeTruthy();
});
