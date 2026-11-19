import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';

const { get } = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('$lib/api', () => ({ api: { get } }));
vi.mock('$lib/state/projects.svelte', () => ({
	projectsState: { currentProjectId: 'project', currentProject: null },
	isFrontendFramework: () => false
}));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));

import SpanTracePage from '../../test/span-trace-page.svelte';

beforeEach(() => {
	vi.stubGlobal(
		'ResizeObserver',
		class {
			observe() {}
			unobserve() {}
			disconnect() {}
		}
	);
});

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});

it.each(['1970-01-01T00:00:00Z', '2554-07-21T23:34:33.709551615Z'])(
	'loads attributes using the stored time when the source start is %s',
	async (startTime) => {
		const recordedAt = '2026-09-20T12:34:56Z';
		get.mockResolvedValueOnce({
			spans: [
				{
					projectId: 'project',
					traceId: 'abc',
					spanId: '01',
					name: 'fallback span',
					startTime,
					recordedAt,
					duration: 1_000_000
				}
			]
		});
		get.mockResolvedValueOnce({ attributes: { 'test.case': 'reopened' } });
		const { findByText } = render(SpanTracePage, {
			data: { traceId: 'abc', at: recordedAt, spanId: null }
		});
		await fireEvent.click(await findByText('fallback span'));
		expect(await findByText('reopened')).toBeTruthy();
		expect(get).toHaveBeenNthCalledWith(1, '/spans/traces/abc?at=2026-09-20T12%3A34%3A56Z', {
			projectId: 'project'
		});
		expect(get).toHaveBeenNthCalledWith(
			2,
			'/spans/traces/abc/spans/01/attributes?at=2026-09-20T12%3A34%3A56Z',
			{ projectId: 'project' }
		);
	}
);
