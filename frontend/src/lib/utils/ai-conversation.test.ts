import { expect, it } from 'vitest';
import {
	buildConversationTimeline,
	extractMessages,
	type ConversationTurn
} from './ai-conversation';

it('shows plain OTLP prompts and completions, including output-only captures', () => {
	expect(extractMessages('请求 café 🚀', 'Review answer')).toEqual([
		{ role: 'user', content: '请求 café 🚀' },
		{ role: 'assistant', content: 'Review answer' }
	]);
	expect(extractMessages('', 'only output')).toEqual([
		{ role: 'assistant', content: 'only output' }
	]);
	expect(extractMessages('', '')).toBeNull();
});

function turn(input: unknown, output: unknown): ConversationTurn {
	return {
		id: 'turn',
		traceName: 'chat',
		recordedAt: '',
		duration: 1,
		model: 'model',
		totalTokens: 1,
		totalCost: 0,
		input: JSON.stringify(input),
		output: JSON.stringify(output)
	};
}

it('merges repeated complete histories without repeating earlier messages', () => {
	const first = { role: 'user', content: 'first question' };
	const answer = { role: 'assistant', content: 'first answer' };
	const next = { role: 'user', content: 'next question' };
	const timeline = buildConversationTimeline([
		turn([first], { choices: [{ message: answer }] }),
		turn([first, answer, next], {
			choices: [{ message: { role: 'assistant', content: 'next answer' } }]
		})
	]);
	expect(
		timeline?.filter((entry) => entry.kind === 'message').map((entry) => entry.message.content)
	).toEqual(['first question', 'first answer', 'next question', 'next answer']);
});

it('falls back to separate turns when history text changes even if its roles match', () => {
	const first = { role: 'user', content: 'original question' };
	const answer = { role: 'assistant', content: 'answer' };
	expect(
		buildConversationTimeline([
			turn([first], { choices: [{ message: answer }] }),
			turn([{ ...first, content: 'replaced question' }, answer], { choices: [{ message: answer }] })
		])
	).toBeNull();
});

it('retains unrecognized response shapes in the per-turn fallback instead of claiming no payload', () => {
	const value = turn([{ role: 'user', content: 'question' }], { vendor_response: 'answer' });
	expect(buildConversationTimeline([value])).toBeNull();
	expect(extractMessages(value.input, value.output)?.[1].content).toContain('vendor_response');
});
