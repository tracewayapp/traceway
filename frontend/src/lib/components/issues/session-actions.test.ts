import { describe, expect, it } from 'vitest';
import {
	actionType,
	clickSummary,
	navigationSummary,
	recordingSequence,
	tapSummary
} from './session-actions';

describe('tapSummary', () => {
	it('reads a tap with its selector, shown text and screen', () => {
		expect(
			tapSummary({
				selector: 'MaterialButton#pay',
				tag: 'MaterialButton',
				id: 'pay',
				text: 'Pay now',
				screen: '/Checkout'
			})
		).toBe('Tap MaterialButton#pay “Pay now” on /Checkout');
	});

	it('leaves out text the selector already names', () => {
		expect(
			tapSummary({
				selector: 'ElevatedButton[text="Pay now"]',
				text: 'Pay now',
				screen: '/checkout'
			})
		).toBe('Tap ElevatedButton[text="Pay now"] on /checkout');
		expect(tapSummary({ selector: 'UIButton[label="Delete"]', label: 'Delete' })).toBe(
			'Tap UIButton[label="Delete"]'
		);
	});

	it('falls back to the label, the tag and the id', () => {
		expect(tapSummary({ selector: '#cart UIButton', label: 'Remove item' })).toBe(
			'Tap #cart UIButton “Remove item”'
		);
		expect(tapSummary({ tag: 'ImageButton', screen: '/CartFragment' })).toBe(
			'Tap ImageButton on /CartFragment'
		);
		expect(tapSummary({ id: 'pay' })).toBe('Tap #pay');
		expect(tapSummary(undefined)).toBe('Tap');
		expect(tapSummary({ selector: 42, text: '  ' })).toBe('Tap');
	});
});

describe('recordingSequence', () => {
	const sequence =
		'click(#open-user) → navigation(/users/{id}) → click(MaterialButton[text="Pay"]) → log(click(here))';

	it('calls a click step a tap in app projects', () => {
		for (const platform of ['flutter', 'ios', 'android'] as const) {
			expect(recordingSequence(sequence, platform)).toBe(
				'tap(#open-user) → navigation(/users/{id}) → tap(MaterialButton[text="Pay"]) → log(click(here))'
			);
		}
	});

	// TW-10: the server names nested steps `(a or b)` and `not a`, and a step's value may hold any text.
	it('calls a nested click step a tap and leaves step values alone', () => {
		expect(recordingSequence('(click(#a) or exception)', 'android')).toBe('(tap(#a) or exception)');
		expect(recordingSequence('(log(x) and click(#b))', 'flutter')).toBe('(log(x) and tap(#b))');
		expect(recordingSequence('not click(#a)', 'ios')).toBe('not tap(#a)');
		expect(recordingSequence('(not click(#a) or click(#b)) → click(#c)', 'ios')).toBe(
			'(not tap(#a) or tap(#b)) → tap(#c)'
		);
		expect(recordingSequence('log(a → click(b))', 'ios')).toBe('log(a → click(b))');
		expect(recordingSequence('log(a(b) → click(c)', 'ios')).toBe('log(a(b) → click(c)');
		expect(recordingSequence('url eq /a → click(#pay)', 'ios')).toBe('url eq /a → tap(#pay)');
	});

	it('keeps clicks in browser projects', () => {
		expect(recordingSequence(sequence, 'browser')).toBe(sequence);
		expect(recordingSequence('hover(#menu)', 'ios')).toBe('hover(#menu)');
	});
});

describe('actionType', () => {
	const click = {
		type: 'custom' as const,
		timestamp: '2026-11-27T05:30:00Z',
		category: 'ui',
		name: 'click'
	};

	it('calls a click row a tap in app projects and a click in browser ones', () => {
		expect(actionType(click, 'flutter')).toBe('tap');
		expect(actionType(click, 'ios')).toBe('tap');
		expect(actionType(click, 'android')).toBe('tap');
		expect(actionType(click, 'browser')).toBe('click');
	});

	it('keeps the type of other rows', () => {
		expect(actionType({ ...click, category: 'event', name: 'checkout' }, 'flutter')).toBe('custom');
		expect(
			actionType(
				{ type: 'navigation', timestamp: click.timestamp, action: 'push', from: '/a', to: '/b' },
				'ios'
			)
		).toBe('navigation');
	});
});

// The browser SDK stamps the page a click was on and the route template of a navigation.
describe('clickSummary and navigationSummary', () => {
	it('reads a browser click like a tap', () => {
		expect(
			clickSummary({ selector: 'button#pay', text: 'Pay now', screen: '/orders/:id' }, 'Click')
		).toBe('Click button#pay “Pay now” on /orders/:id');
		expect(clickSummary({ selector: 'a.logo' }, 'Click')).toBe('Click a.logo');
	});

	it('adds the route template when it says more than the path', () => {
		expect(
			navigationSummary({
				type: 'navigation',
				timestamp: '2026-11-27T10:00:00Z',
				action: 'push',
				from: '/orders',
				to: '/orders/42',
				route: '/orders/:id'
			})
		).toBe('push: /orders → /orders/42 (/orders/:id)');
		expect(
			navigationSummary({
				type: 'navigation',
				timestamp: '2026-11-27T10:00:00Z',
				action: 'pop',
				to: '/CartActivity',
				route: '/CartActivity'
			})
		).toBe('pop: ? → /CartActivity');
		expect(
			navigationSummary({ type: 'navigation', timestamp: '', action: 5, to: null } as never)
		).toBe('navigate: ? → ?');
	});
});
