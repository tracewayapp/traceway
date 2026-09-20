import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));
import SpanWaterfall from './span-waterfall.svelte';
import type { Span } from '$lib/types/spans';

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
	vi.unstubAllGlobals();
});

it('collapses descendants and keeps siblings visible, then restores the subtree', async () => {
	const makeSpan = (id: string, parentSpanId?: string): Span => ({
		spanId: id,
		parentSpanId,
		traceId: 'trace',
		projectId: 'project',
		name: `span ${id}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000_000
	});
	const { getAllByRole, getByRole, queryByText, getByText } = render(SpanWaterfall, {
		spans: [makeSpan('01'), makeSpan('02', '01'), makeSpan('03', '02'), makeSpan('04')],
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	expect(getAllByRole('button', { name: 'Collapse children' })).toHaveLength(2);
	await fireEvent.click(getAllByRole('button', { name: 'Collapse children' })[0]);
	expect(queryByText('span 02')).toBeNull();
	expect(queryByText('span 03')).toBeNull();
	expect(getByText('span 04')).toBeTruthy();
	const expand = getByRole('button', { name: 'Expand children' });
	expect(expand.getAttribute('aria-expanded')).toBe('false');
	await fireEvent.click(expand);
	expect(getByText('span 02')).toBeTruthy();
	expect(getByText('span 03')).toBeTruthy();
	expect(getAllByRole('button', { name: 'Collapse children' })).toHaveLength(2);
});

it('marks errors, missing parents, the selected span, and names services once there are several', () => {
	const makeSpan = (id: string, overrides: Partial<Span>): Span => ({
		spanId: id,
		traceId: 'trace',
		projectId: 'project',
		name: `span ${id}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000_000,
		...overrides
	});
	const { getAllByLabelText, getByText, container } = render(SpanWaterfall, {
		spans: [
			makeSpan('01', { serviceName: 'web', statusCode: 2 }),
			makeSpan('02', { serviceName: 'api', parentSpanId: '99' })
		],
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000,
		selectedSpanId: '02'
	});
	expect(getAllByLabelText('Error')).toHaveLength(1);
	expect(getAllByLabelText('Parent span is unavailable')).toHaveLength(1);
	expect(getByText('web')).toBeTruthy();
	expect(getByText('api')).toBeTruthy();
	const selected = container.querySelectorAll('[aria-current="true"]');
	expect(selected).toHaveLength(1);
	expect(selected[0].textContent).toContain('span 02');
});

it('leaves the service name out when the whole trace is one service', () => {
	const { queryByText } = render(SpanWaterfall, {
		spans: [
			{
				spanId: '01',
				traceId: 'trace',
				projectId: 'project',
				name: 'only span',
				serviceName: 'web',
				startTime: '2026-09-17T00:00:00Z',
				recordedAt: '',
				duration: 1_000_000
			}
		],
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	expect(queryByText('web')).toBeNull();
});

it('pages a very large trace and links to the whole trace', async () => {
	const spans: Span[] = Array.from({ length: 620 }, (_, i) => ({
		spanId: i.toString(16).padStart(16, '0'),
		traceId: 'abc',
		projectId: 'project',
		name: `span ${i}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000_000 + i
	}));
	const { getByRole, queryByText, getByText } = render(SpanWaterfall, {
		spans,
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	expect(queryByText('span 619')).toBeNull();
	await fireEvent.click(getByRole('button', { name: /Show 120 more/ }));
	expect(getByText('span 619')).toBeTruthy();
	expect(getByRole('link', { name: 'Open the whole trace' }).getAttribute('href')).toContain(
		'/spans/abc?at='
	);
}, 30_000);

it('links the whole trace by the trace id and start time of the page it sits on', () => {
	const { getByRole } = render(SpanWaterfall, {
		spans: [
			{
				spanId: '02',
				parentSpanId: '01',
				traceId: 'abc',
				projectId: 'project',
				name: 'child',
				startTime: '2026-09-17T00:00:00.250Z',
				recordedAt: '',
				duration: 1_000_000
			}
		],
		traceId: 'abc',
		rootSpanId: '01',
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	expect(getByRole('link', { name: 'Open the whole trace' }).getAttribute('href')).toBe(
		'/spans/abc?at=2026-09-17T00%3A00%3A00Z'
	);
});

it('has no whole-trace link inside the trace view itself', () => {
	const { queryByRole } = render(SpanWaterfall, {
		spans: [
			{
				spanId: '01',
				traceId: 'abc',
				projectId: 'project',
				name: 'only span',
				startTime: '2026-09-17T00:00:00Z',
				recordedAt: '',
				duration: 1_000_000
			}
		],
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000,
		linkTraces: false
	});
	expect(queryByRole('link', { name: 'Open the whole trace' })).toBeNull();
});

function wideTrace(): Span[] {
	const make = (id: number, parent?: number, statusCode?: number): Span => ({
		spanId: id.toString(16).padStart(16, '0'),
		parentSpanId: parent === undefined ? undefined : parent.toString(16).padStart(16, '0'),
		traceId: 'abc',
		projectId: 'project',
		name: `span ${id}`,
		startTime: '2026-09-17T00:00:00Z',
		recordedAt: '',
		duration: 1_000_000,
		statusCode
	});
	return [
		make(1),
		make(2, 1),
		make(3, 1),
		...Array.from({ length: 600 }, (_, i) => make(i + 100, 2, i === 7 ? 2 : 0))
	];
}

it('opens a large trace with its widest node folded into a pill that flags the error inside', async () => {
	const { getByRole, getByText, queryByText, getAllByLabelText } = render(SpanWaterfall, {
		spans: wideTrace(),
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	expect(getByText('span 3')).toBeTruthy();
	expect(queryByText('span 100')).toBeNull();
	const pill = getByRole('button', { name: 'Show 600 hidden spans' });
	expect(pill.textContent).toContain('+600');
	expect(getAllByLabelText('Error in the hidden spans')).toHaveLength(1);
	await fireEvent.click(pill);
	expect(getByText('span 100')).toBeTruthy();
});

it('collapses and expands everything from the header', async () => {
	const { getByRole, getByText, queryByText } = render(SpanWaterfall, {
		spans: wideTrace(),
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000
	});
	await fireEvent.click(getByRole('button', { name: 'Collapse all' }));
	expect(getByText('span 1')).toBeTruthy();
	expect(queryByText('span 2')).toBeNull();
	expect(getByRole('button', { name: 'Show 602 hidden spans' })).toBeTruthy();
	await fireEvent.click(getByRole('button', { name: 'Expand all' }));
	expect(getByText('span 2')).toBeTruthy();
	expect(getByText('span 100')).toBeTruthy();
});

it("asks for a span's attributes once, when its popover opens", async () => {
	const loadAttributes = vi.fn(async () => ({ attributes: { 'url.path': '/checkout' } }));
	const { getByText, getAllByText, findByText } = render(SpanWaterfall, {
		spans: [
			{
				spanId: '01',
				traceId: 'abc',
				projectId: 'project',
				name: 'lazy span',
				startTime: '2026-09-17T00:00:00Z',
				recordedAt: '',
				duration: 1_000_000
			}
		],
		traceStartTime: '2026-09-17T00:00:00Z',
		traceDuration: 5_000_000,
		loadAttributes
	});
	expect(loadAttributes).not.toHaveBeenCalled();
	await fireEvent.click(getByText('lazy span'));
	expect(await findByText('/checkout')).toBeTruthy();
	// The popover repeats the name, so the row label is the first match.
	await fireEvent.click(getAllByText('lazy span')[0]);
	await fireEvent.click(getAllByText('lazy span')[0]);
	expect(loadAttributes).toHaveBeenCalledTimes(1);
});
