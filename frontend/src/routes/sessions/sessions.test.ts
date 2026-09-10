import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';

const { post, goto } = vi.hoisted(() => ({ post: vi.fn(), goto: vi.fn() }));
vi.mock('$lib/api', () => ({ api: { post } }));
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
afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	if (scrollIntoViewDescriptor) {
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', scrollIntoViewDescriptor);
	} else {
		Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
	}
});

it('loads old attribute links, edits and removes chips, and combines text search', async () => {
	render(SessionsPage);
	await screen.findByRole('button', { name: 'Edit filter' });
	await waitFor(() =>
		expect(post).toHaveBeenCalledWith(
			'/sessions',
			expect.objectContaining({
				attributeFilters: [{ key: 'userId', value: 'u_42', exclude: false, contains: false }]
			}),
			{ projectId: 'project-1' }
		)
	);
	await fireEvent.click(screen.getByRole('button', { name: 'Edit filter' }));
	const value = await screen.findByPlaceholderText('u_42');
	await fireEvent.input(value, { target: { value: 'u_43' } });
	await fireEvent.keyDown(
		within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Equals' }),
		{ key: 'ArrowDown' }
	);
	await fireEvent.pointerUp(await screen.findByRole('option', { name: 'Contains' }), {
		pointerType: 'mouse'
	});
	await fireEvent.click(screen.getByRole('button', { name: 'Update filter' }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1].attributeFilters[0]).toMatchObject({
			value: 'u_43',
			contains: true,
			exclude: false
		})
	);
	expect(new URLSearchParams(window.location.search).get('attr')).toBe('userId~=u_43');
	await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
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
	await fireEvent.click(screen.getByRole('button', { name: 'Remove filter' }));
	await waitFor(() =>
		expect(post.mock.lastCall?.[1]).toMatchObject({ search: 'alice', attributeFilters: [] })
	);
});

it('opens an attribute filter from a row without navigating to the session', async () => {
	render(SessionsPage);
	const attribute = await screen.findByRole('button', {
		name: 'Filter by email=alice@example.com'
	});
	goto.mockClear();
	await fireEvent.click(attribute);
	expect(goto).not.toHaveBeenCalled();
	expect(await screen.findByDisplayValue('email')).toBeTruthy();
	expect(screen.getByDisplayValue('alice@example.com')).toBeTruthy();
});
