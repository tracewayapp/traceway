import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';

const { post, get, goto } = vi.hoisted(() => ({ post: vi.fn(), get: vi.fn(), goto: vi.fn() }));
vi.mock('$lib/api', () => ({ api: { post, get } }));
vi.mock('$lib/state/projects.svelte', () => ({ projectsState: { currentProjectId: 'project-1' } }));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));
vi.mock('$app/navigation', () => ({ goto }));
vi.mock('$app/environment', () => ({ browser: true }));

import SessionsPage from '../../test/sessions-page.svelte';

const scrollIntoViewDescriptor = Object.getOwnPropertyDescriptor(
	HTMLElement.prototype,
	'scrollIntoView'
);

beforeEach(() => {
	Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
		configurable: true,
		value: vi.fn()
	});
	window.history.replaceState({}, '', '/sessions?preset=24h&attr=userId%3Du_42');
	goto.mockImplementation(async (url: string) => {
		window.history.replaceState({}, '', url);
	});
	get.mockResolvedValue({
		reasons: [
			{ triggerId: 'journey', name: 'User journey', sequence: 'click(#open)', revision: 'v1' }
		]
	});
	post.mockResolvedValue({
		data: [
			{
				id: 'a5100000-0000-4000-8000-000000000000',
				startedAt: new Date().toISOString(),
				duration: 0,
				attributes: { userId: 'u_42', email: 'alice@example.com' }
			}
		],
		pagination: { total: 1, totalPages: 1 }
	});
});
afterEach(async () => {
	cleanup();
	// bits-ui resets the body scroll lock 24ms after a dialog unmounts; if jsdom is torn
	// down first, that timer throws "document is not defined".
	await new Promise((resolve) => setTimeout(resolve, 50));
	vi.clearAllMocks();
	if (scrollIntoViewDescriptor) {
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', scrollIntoViewDescriptor);
	} else {
		Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
	}
});

it('loads old attribute links, edits a pill in the drawer, and combines text search', async () => {
	render(SessionsPage);
	const pill = await screen.findByRole('button', { name: 'Edit filter userId = u_42' });
	await waitFor(() =>
		expect(post).toHaveBeenCalledWith(
			'/sessions',
			expect.objectContaining({
				attributeFilters: [
					{ key: 'userId', value: 'u_42', exclude: false, contains: false, exists: false }
				]
			}),
			{ projectId: 'project-1' }
		)
	);
	await fireEvent.click(pill);
	const drawer = await screen.findByRole('dialog');
	const value = within(drawer).getByDisplayValue('u_42');
	await fireEvent.input(value, { target: { value: 'u_43' } });
	await fireEvent.keyDown(within(drawer).getByRole('button', { name: 'Operator' }), {
		key: 'ArrowDown'
	});
	await fireEvent.pointerUp(await screen.findByRole('option', { name: '~ Contains' }), {
		pointerType: 'mouse'
	});
	await fireEvent.click(within(drawer).getByRole('button', { name: 'Update filter' }));
	// Edits wait in the drawer until Apply.
	expect(post.mock.lastCall?.[1].attributeFilters[0]).toMatchObject({ value: 'u_42' });
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1].attributeFilters[0]).toMatchObject({
			value: 'u_43',
			contains: true,
			exclude: false
		})
	);
	expect(new URLSearchParams(window.location.search).get('attr')).toBe('userId~=u_43');
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
	await fireEvent.input(
		screen.getByPlaceholderText('Search session ID, user, or attribute value...'),
		{ target: { value: 'alice' } }
	);
	await fireEvent.click(screen.getByRole('button', { name: 'Go' }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1]).toMatchObject({
			search: 'alice',
			attributeFilters: [{ key: 'userId', value: 'u_43' }]
		})
	);
	await fireEvent.click(screen.getByRole('button', { name: 'Remove filter userId' }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1]).toMatchObject({ search: 'alice', attributeFilters: [] })
	);
});

it('opens the drawer with a row attribute filled in, without navigating', async () => {
	render(SessionsPage);
	const attribute = await screen.findByRole('button', {
		name: 'Filter by email=alice@example.com'
	});
	goto.mockClear();
	const calls = post.mock.calls.length;
	await fireEvent.click(attribute);
	expect(goto).not.toHaveBeenCalled();
	const drawer = await screen.findByRole('dialog');
	expect(within(drawer).getByDisplayValue('email')).toBeTruthy();
	expect(within(drawer).getByDisplayValue('alice@example.com')).toBeTruthy();
	expect(post.mock.calls.length).toBe(calls);
	await fireEvent.click(within(drawer).getByRole('button', { name: 'Add filter' }));
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1].attributeFilters).toEqual([
			{ key: 'userId', value: 'u_42', exclude: false, contains: false, exists: false },
			{ key: 'email', value: 'alice@example.com', exclude: false, contains: false, exists: false }
		])
	);
});

it('round-trips the recording trigger through the URL, a pill and the drawer', async () => {
	window.history.replaceState({}, '', '/sessions?preset=24h&triggerId=journey');
	render(SessionsPage);
	await waitFor(() => expect(post.mock.lastCall?.[1].triggerId).toBe('journey'));
	await screen.findByRole('button', { name: 'Edit filter Recording trigger = User journey' });
	await fireEvent.click(screen.getByRole('button', { name: 'Remove Recording trigger filter' }));
	await waitFor(() => expect(post.mock.lastCall?.[1].triggerId).toBe(''));
	expect(new URLSearchParams(window.location.search).has('triggerId')).toBe(false);

	await fireEvent.click(screen.getByRole('button', { name: 'Add filter' }));
	const drawer = await screen.findByRole('dialog');
	await fireEvent.keyDown(within(drawer).getByRole('button', { name: 'Recording trigger' }), {
		key: 'ArrowDown'
	});
	await fireEvent.pointerUp(await screen.findByRole('option', { name: 'User journey' }), {
		pointerType: 'mouse'
	});
	await fireEvent.click(within(drawer).getByRole('button', { name: /^Apply/ }));
	await waitFor(() => expect(post.mock.lastCall?.[1].triggerId).toBe('journey'));
	expect(new URLSearchParams(window.location.search).get('triggerId')).toBe('journey');
});
