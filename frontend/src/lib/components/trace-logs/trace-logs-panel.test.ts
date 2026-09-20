import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { api } from '$lib/api';
import TraceLogsPanel from './trace-logs-panel.svelte';

vi.mock('$lib/api', () => ({ api: { post: vi.fn() } }));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

it('names logs emitted by the occurrence root when the child graph excludes that span', async () => {
	vi.mocked(api.post).mockResolvedValue({
		data: [
			{
				id: 'root-log',
				body: 'root message',
				spanId: 'ABCDEF0123456789',
				timestamp: '2026-09-01T12:00:00Z'
			}
		]
	});
	const { findByText } = render(TraceLogsPanel, {
		projectId: 'project-a',
		traceId: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		spans: [],
		rootSpan: { spanId: 'abcdef0123456789', name: 'GET /root' },
		traceRecordedAt: '2026-09-01T12:00:00Z'
	});
	expect(await findByText('GET /root')).toBeTruthy();
});

it('loads related trace logs lazily with the selected project and bounded time window', async () => {
	vi.mocked(api.post).mockResolvedValue({ data: [] });
	const traceId = '1234567890abcdef1234567890abcdef';
	const { getByRole } = render(TraceLogsPanel, {
		projectId: 'selected-project',
		traceId,
		spans: [],
		traceRecordedAt: '2026-09-01T12:00:00Z'
	});
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1));
	expect(vi.mocked(api.post).mock.calls[0][1]).toEqual(expect.objectContaining({ traceId }));

	await fireEvent.click(getByRole('tab', { name: 'Related Traces' }));
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
	expect(api.post).toHaveBeenLastCalledWith(
		'/logs',
		expect.objectContaining({
			distributedTraceId: traceId,
			fromDate: '2026-09-01T11:00:00.000Z',
			toDate: '2026-09-01T13:00:00.000Z',
			pagination: { page: 1, pageSize: 100 }
		}),
		{ projectId: 'selected-project' }
	);
	await fireEvent.click(getByRole('tab', { name: 'This Trace' }));
	await fireEvent.click(getByRole('tab', { name: 'Related Traces' }));
	expect(api.post).toHaveBeenCalledTimes(2);
});

it('reloads when trace props change and ignores a late response from the old trace', async () => {
	let finishOld!: (value: unknown) => void;
	vi.mocked(api.post).mockImplementationOnce(
		() =>
			new Promise((resolve) => {
				finishOld = resolve;
			})
	);
	vi.mocked(api.post).mockResolvedValue({ data: [] });
	const { rerender, queryByText, findByText } = render(TraceLogsPanel, {
		projectId: 'project-a',
		traceId: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		spans: [],
		traceRecordedAt: '2026-09-01T12:00:00Z'
	});
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(1));
	await rerender({ projectId: 'project-b', traceId: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' });
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
	finishOld({
		data: [{ id: 'old-log', body: 'stale trace log', timestamp: '2026-09-01T12:00:00Z' }]
	});
	expect(await findByText('No logs for this trace in the surrounding time window')).toBeTruthy();
	expect(queryByText('stale trace log')).toBeNull();
	expect(api.post).toHaveBeenLastCalledWith(
		'/logs',
		expect.objectContaining({ traceId: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' }),
		{ projectId: 'project-b' }
	);
});

it('invalidates related logs and their default trace ID on navigation', async () => {
	vi.mocked(api.post).mockResolvedValue({ data: [] });
	const { rerender, getByRole } = render(TraceLogsPanel, {
		projectId: 'project-a',
		traceId: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		spans: [],
		traceRecordedAt: '2026-09-01T12:00:00Z'
	});
	await fireEvent.click(getByRole('tab', { name: 'Related Traces' }));
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(2));
	await rerender({
		traceId: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
		traceRecordedAt: '2026-09-02T12:00:00Z'
	});
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(3));
	expect(getByRole('tab', { name: 'This Trace' }).getAttribute('aria-selected')).toBe('true');
	await fireEvent.click(getByRole('tab', { name: 'Related Traces' }));
	await waitFor(() => expect(api.post).toHaveBeenCalledTimes(4));
	expect(api.post).toHaveBeenLastCalledWith(
		'/logs',
		expect.objectContaining({
			distributedTraceId: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
			fromDate: '2026-09-02T11:00:00.000Z'
		}),
		{ projectId: 'project-a' }
	);
});

it('pages beyond the first 100 logs instead of silently hiding them', async () => {
	vi.mocked(api.post).mockResolvedValue({ data: [], pagination: { total: 101 } });
	const { getByRole, findByText } = render(TraceLogsPanel, {
		projectId: 'project-a',
		traceId: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		spans: [],
		traceRecordedAt: '2026-09-01T12:00:00Z'
	});
	expect(await findByText('Page 1 of 2 · 101 logs')).toBeTruthy();
	await fireEvent.click(getByRole('button', { name: 'Next' }));
	expect(await findByText('Page 2 of 2 · 101 logs')).toBeTruthy();
	expect(api.post).toHaveBeenLastCalledWith(
		'/logs',
		expect.objectContaining({ pagination: { page: 2, pageSize: 100 } }),
		{ projectId: 'project-a' }
	);
	expect(getByRole('button', { name: 'Next' }).hasAttribute('disabled')).toBe(true);
});
