import { expect, it } from 'vitest';
import {
	buildConversationTimeline,
	extractMessages,
	pairToolResults,
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

it('falls back when a resent prompt changes the order of image and text blocks', () => {
	const text = { type: 'text', text: 'Describe this.' };
	const image = { type: 'image_url', image_url: { url: 'example.png' } };
	const answer = { role: 'assistant', content: 'A tree.' };
	const completion = { choices: [{ message: answer }] };
	expect(
		buildConversationTimeline([
			turn([{ role: 'user', content: [image, text] }], completion),
			turn([{ role: 'user', content: [text, image] }, answer], completion)
		])
	).toBeNull();
});

it('retains unrecognized response shapes in the per-turn fallback instead of claiming no payload', () => {
	const value = turn([{ role: 'user', content: 'question' }], { vendor_response: 'answer' });
	expect(buildConversationTimeline([value])).toBeNull();
	expect(extractMessages(value.input, value.output)?.[1].content).toContain('vendor_response');
});

it('merges Anthropic histories, where an assistant turn comes back as content blocks', () => {
	const question = { role: 'user', content: 'How do webhooks retry?' };
	const toolUse = {
		type: 'tool_use',
		id: 'toolu_1',
		name: 'search_docs',
		input: { query: 'webhooks' }
	};
	const firstAnswer = {
		role: 'assistant',
		content: [
			{ type: 'text', text: 'Let me look that up.' },
			toolUse,
			{ ...toolUse, name: 'lookup_policy', id: 'toolu_2' }
		]
	};
	const toolResult = {
		role: 'user',
		content: [
			{ type: 'tool_result', tool_use_id: 'toolu_2', content: 'Retries last 24 hours.' },
			{ type: 'tool_result', tool_use_id: 'toolu_1', content: 'See the retry policy.' },
			{ type: 'text', text: 'Summarize both results.' }
		]
	};
	const timeline = buildConversationTimeline([
		turn({ messages: [question] }, { ...firstAnswer, type: 'message', stop_reason: 'tool_use' }),
		turn(
			{ messages: [question, firstAnswer, toolResult] },
			{ role: 'assistant', content: [{ type: 'text', text: 'They retry for 24 hours.' }] }
		)
	]);
	const messages = (timeline ?? []).flatMap((entry) =>
		entry.kind === 'message' ? [entry.message] : []
	);
	expect(messages.map((message) => message.role)).toEqual([
		'user',
		'assistant',
		'tool',
		'tool',
		'user',
		'assistant'
	]);
	const calls = messages[1].toolCalls ?? [];
	expect(calls.map((call) => call.name)).toEqual(['search_docs', 'lookup_policy']);
	const { byCall, pairedMessages } = pairToolResults(messages);
	expect(byCall.get(calls[0])?.content).toBe('See the retry policy.');
	expect(byCall.get(calls[1])?.content).toBe('Retries last 24 hours.');
	expect([...pairedMessages]).toEqual(messages.slice(2, 4));
	expect(messages[4].content).toBe('Summarize both results.');
});
