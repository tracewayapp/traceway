import { describe, expect, it } from 'vitest';
import { spanDisplayName } from './span-display';
import type { Span } from '$lib/types/spans';

function span(overrides: Partial<Span>): Span {
	return {
		traceId: 'trace',
		spanId: '01',
		projectId: 'project',
		name: 'SELECT shop.orders',
		startTime: '2026-09-17T00:00:00Z',
		duration: 100,
		recordedAt: '',
		...overrides
	};
}

describe('spanDisplayName', () => {
	it('previews the statement of a database span and prefers db.query.text', () => {
		const attributes = { 'db.query.text': 'SELECT 1', 'db.statement': 'SELECT 2' };
		expect(spanDisplayName(span({ attributes }))).toBe('SELECT 1');
		expect(spanDisplayName(span({ attributes: { 'db.statement': 'SELECT 2' } }))).toBe('SELECT 2');
	});

	it('keeps the span name when there is no statement', () => {
		expect(spanDisplayName(span({}))).toBe('SELECT shop.orders');
		expect(spanDisplayName(span({ attributes: null }))).toBe('SELECT shop.orders');
		expect(spanDisplayName(span({ attributes: { 'db.query.text': '  \n ' } }))).toBe(
			'SELECT shop.orders'
		);
	});

	it('collapses whitespace and bounds a very long statement', () => {
		const statement = `SELECT *\n\tFROM orders\n WHERE id IN (${'1, '.repeat(500)}2)`;
		const preview = spanDisplayName(span({ attributes: { 'db.query.text': statement } }));
		expect(preview.startsWith('SELECT * FROM orders WHERE id IN (1, 1,')).toBe(true);
		expect(preview.length).toBe(201);
		expect(preview.endsWith('…')).toBe(true);
	});
});

describe('spanDisplayName without attributes', () => {
	it('uses the statement preview a whole trace read sends along', () => {
		expect(spanDisplayName(span({ dbStatement: 'SELECT  *\n FROM orders' }))).toBe(
			'SELECT * FROM orders'
		);
	});
});
