import { cleanup, render } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterEach, expect, it, vi } from 'vitest';
import Dashboard from './+page.svelte';
import { scheduleEmbedExpiry } from '$lib/utils/dashboard-embed';

vi.mock('$lib/components/dashboard/widget-grid.svelte', () => ({ default: vi.fn() }));

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	history.replaceState(null, '', '/');
});

it('expires the displayed view while a renewed link is waiting for its dashboard', async () => {
	vi.useFakeTimers();
	history.replaceState(null, '', '/embed/dashboard?dashboard=orders#token=first');
	const metadata = {
		name: 'Orders',
		from: '2026-11-20T00:00:00Z',
		to: '2026-11-21T00:00:00Z',
		expiresInSeconds: 5,
		widgets: []
	};
	let resolveRenewal!: (response: Response) => void;
	const fetch = vi
		.fn()
		.mockResolvedValueOnce(Response.json(metadata))
		.mockImplementationOnce(() => new Promise<Response>((resolve) => (resolveRenewal = resolve)));
	vi.stubGlobal('fetch', fetch);
	const page = render(Dashboard);
	await vi.advanceTimersByTimeAsync(0);
	expect(page.getByRole('heading', { name: 'Orders' })).toBeTruthy();

	history.replaceState(null, '', '/embed/dashboard?dashboard=orders#token=second');
	window.dispatchEvent(new HashChangeEvent('hashchange'));
	await tick();
	expect(fetch).toHaveBeenCalledTimes(2);
	await vi.advanceTimersByTimeAsync(5000);
	expect(page.queryByRole('heading')).toBeNull();
	expect(page.getByRole('alert').textContent).toContain('expired');

	resolveRenewal(Response.json({ ...metadata, name: 'Renewed orders' }));
	await vi.advanceTimersByTimeAsync(0);
	expect(page.getByRole('heading', { name: 'Renewed orders' })).toBeTruthy();
	expect(page.queryByRole('alert')).toBeNull();
});

it('keeps a 30-day link active across the browser timer limit', async () => {
	vi.useFakeTimers();
	const expire = vi.fn();
	const day = 86400;
	const cancel = scheduleEmbedExpiry(30 * day, expire);
	await vi.advanceTimersByTimeAsync(29 * day * 1000);
	expect(expire).not.toHaveBeenCalled();
	await vi.advanceTimersByTimeAsync(day * 1000);
	expect(expire).toHaveBeenCalledTimes(1);
	cancel();
});
