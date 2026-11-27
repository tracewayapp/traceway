import type { SdkPlatform } from '$lib/components/sdk-config/sdk-config';
import type { SessionActionEvent, SessionNavigationEvent } from '$lib/types/exceptions';

function text(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

/**
 * A tap from an app SDK (Flutter, iOS, Android) in one line: the recorded selector, what the control shows when
 * the selector does not already say it, and the screen, e.g. `Tap MaterialButton#pay “Pay now” on /Checkout`.
 * The data is a click row of the session timeline: `selector`, `tag`, `id`, `text`, `label`, `screen`.
 */
export function tapSummary(data: Record<string, unknown> | undefined): string {
	return clickSummary(data, 'Tap');
}

/**
 * A click row in one line, `Tap …` from an app SDK and `Click …` from the browser SDK, which stamps the page it was
 * on in the same `screen` field.
 */
export function clickSummary(
	data: Record<string, unknown> | undefined,
	verb: 'Tap' | 'Click'
): string {
	const selector =
		text(data?.selector) || text(data?.tag) || (text(data?.id) && `#${text(data?.id)}`);
	const parts: string[] = [verb];
	if (selector) parts.push(selector);
	const shown = text(data?.text) || text(data?.label);
	if (shown && !selector.includes(shown)) parts.push(`“${shown}”`);
	const screen = text(data?.screen);
	if (screen) parts.push(`on ${screen}`);
	return parts.join(' ');
}

/**
 * A navigation in one line, with the route template of the screen or page it reached when the SDK knows it and it
 * says more than the path: `push: /orders → /orders/42 (/orders/:id)`.
 */
export function navigationSummary(n: SessionNavigationEvent): string {
	const from = text(n.from) || '?';
	const to = text(n.to) || '?';
	const route = text(n.route);
	return `${text(n.action) || 'navigate'}: ${from} → ${to}${route && route !== to ? ` (${route})` : ''}`;
}

/**
 * The Type column of a session action: a click row is a "tap" from an app SDK and a "click" from the browser SDK,
 * not one more "custom" action; the other rows keep their type.
 */
export function actionType(action: SessionActionEvent, platform: SdkPlatform): string {
	if (action.type === 'custom' && action.category === 'ui' && action.name === 'click')
		return platform === 'browser' ? 'click' : 'tap';
	return action.type;
}

// Where the server's step labels start another step: a sequence joins its steps with an arrow, a combination its
// parts with `or`/`and` inside parentheses, and a negation prefixes `not`.
const STEP_SEPARATORS = [' → ', ' or ', ' and '];
const EVENT_STEP = /^[A-Za-z_][\w.]*\(/;

/**
 * A recording reason's steps in the project's words: the server stores a step as its event type, and a click
 * step of an app SDK is a tap. An event step reads `type(value)`, and its value is any text, `click(` and arrows
 * included, so it is skipped to its closing parenthesis rather than rewritten.
 */
export function recordingSequence(sequence: string, platform: SdkPlatform): string {
	if (platform === 'browser') return sequence;
	let out = '';
	let i = 0;
	let stepStart = true;
	while (i < sequence.length) {
		if (stepStart && sequence[i] === '(') {
			out += '(';
			i++;
			continue;
		}
		if (stepStart && sequence.startsWith('not ', i)) {
			out += 'not ';
			i += 4;
			continue;
		}
		const event = stepStart ? EVENT_STEP.exec(sequence.slice(i)) : null;
		if (event) {
			let end = i + event[0].length;
			for (let depth = 1; end < sequence.length && depth > 0; end++) {
				if (sequence[end] === '(') depth++;
				else if (sequence[end] === ')') depth--;
			}
			const type = event[0].slice(0, -1);
			out += (type === 'click' ? 'tap' : type) + sequence.slice(i + type.length, end);
			i = end;
			stepStart = false;
			continue;
		}
		stepStart = false;
		const separator = STEP_SEPARATORS.find((candidate) => sequence.startsWith(candidate, i));
		if (separator) {
			out += separator;
			i += separator.length;
			stepStart = true;
			continue;
		}
		out += sequence[i];
		i++;
	}
	return out;
}
