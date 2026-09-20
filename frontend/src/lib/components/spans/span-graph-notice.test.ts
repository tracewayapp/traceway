import { afterEach, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/svelte';
import SpanGraphNotice from './span-graph-notice.svelte';

afterEach(cleanup);

it('does not warn for a complete or legacy response', () => {
	const { queryByRole, rerender } = render(SpanGraphNotice, { status: { state: 'complete' } });
	expect(queryByRole('alert')).toBeNull();
	return rerender({ status: undefined }).then(() => expect(queryByRole('alert')).toBeNull());
});

it('identifies a truncated graph', () => {
	const { getByRole } = render(SpanGraphNotice, {
		status: { state: 'partial', reasons: ['row_limit'] }
	});
	expect(getByRole('alert').textContent?.replace(/\s+/g, ' ')).toContain('only part of the trace');
});

it('explains omitted attributes', () => {
	const { getByRole } = render(SpanGraphNotice, {
		status: { state: 'partial', reasons: ['attribute_size'], omittedAttributes: 2 }
	});
	expect(getByRole('alert').textContent?.replace(/\s+/g, ' ')).toContain(
		'Attributes are missing from 2 spans'
	);
});

it('distinguishes an unavailable graph from a trace with no spans', () => {
	const { getByRole } = render(SpanGraphNotice, {
		status: { state: 'unavailable', reasons: ['read_limit'] }
	});
	expect(getByRole('alert').textContent?.replace(/\s+/g, ' ')).toContain(
		'Spans are temporarily unavailable'
	);
});

it('says which spans were kept when a whole trace is over the cap', () => {
	const { getByRole } = render(SpanGraphNotice, {
		status: { state: 'partial', reasons: ['row_limit', 'most_important'] }
	});
	expect(getByRole('alert').textContent?.replace(/\s+/g, ' ')).toContain(
		'errors, entry points and slowest spans'
	);
});
