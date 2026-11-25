export type ContentPart = {
	type: string;
	text?: string;
	image_url?: { url: string };
};

export type ToolCall = {
	id?: string;
	name: string;
	arguments: string;
};

export type ChatMessage = {
	role: string;
	content: string | ContentPart[];
	toolCalls?: ToolCall[];
	toolCallId?: string;
};

export type ConversationTurn = {
	id: string;
	traceName: string;
	recordedAt: string;
	duration: number;
	model: string;
	totalTokens: number;
	totalCost: number;
	input: string;
	output: string;
};

export type TimelineEntry =
	| { kind: 'message'; message: ChatMessage }
	| { kind: 'turn-meta'; turn: ConversationTurn };

type JsonRecord = Record<string, unknown>;

function isRecord(value: unknown): value is JsonRecord {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function asString(value: unknown): string | undefined {
	return typeof value === 'string' ? value : undefined;
}

export function tryParseJson(str: string): unknown {
	try {
		return JSON.parse(str);
	} catch {
		return null;
	}
}

function normalizeOpenAiToolCalls(rawCalls: unknown): ToolCall[] | undefined {
	if (!Array.isArray(rawCalls) || rawCalls.length === 0) return undefined;
	const calls: ToolCall[] = [];
	for (const value of rawCalls) {
		if (!isRecord(value)) continue;
		const fn = isRecord(value.function) ? value.function : undefined;
		const name = asString(fn?.name) ?? asString(value.name);
		if (!name) continue;
		const args = fn?.arguments ?? value.arguments ?? '';
		calls.push({
			id: asString(value.id),
			name,
			arguments: typeof args === 'string' ? args : JSON.stringify(args)
		});
	}
	return calls.length > 0 ? calls : undefined;
}

function normalizeMessage(value: unknown): ChatMessage | null {
	if (!isRecord(value)) return null;
	const role = asString(value.role);
	if (!role) return null;

	// Anthropic assistant messages carry tool calls alongside text blocks.
	if (Array.isArray(value.content)) {
		const toolCalls: ToolCall[] = [];
		const toolCallId = asString(value.tool_call_id);
		const parts: ContentPart[] = [];
		for (const item of value.content) {
			if (!isRecord(item)) continue;
			const type = asString(item.type);
			if (type === 'tool_use' && asString(item.name)) {
				toolCalls.push({
					id: asString(item.id),
					name: asString(item.name)!,
					arguments: JSON.stringify(item.input ?? {})
				});
			} else if (type) {
				const image = isRecord(item.image_url) ? asString(item.image_url.url) : undefined;
				parts.push({
					...item,
					type,
					text: asString(item.text),
					image_url: image ? { url: image } : undefined
				});
			}
		}
		const message: ChatMessage = {
			role,
			content: parts.length === 1 && parts[0].type === 'text' ? (parts[0].text ?? '') : parts
		};
		if (toolCalls.length > 0) message.toolCalls = toolCalls;
		if (toolCallId) message.toolCallId = toolCallId;
		return message;
	}

	const toolCalls = normalizeOpenAiToolCalls(value.tool_calls);
	if (value.content === undefined && !toolCalls) return null;
	return {
		role,
		content: typeof value.content === 'string' ? value.content : '',
		toolCalls,
		toolCallId: asString(value.tool_call_id)
	};
}

function extractOutputMessage(output: string): ChatMessage | null {
	const outputParsed = tryParseJson(output);
	if (!isRecord(outputParsed)) {
		return output.trim()
			? { role: 'assistant', content: typeof outputParsed === 'string' ? outputParsed : output }
			: null;
	}

	// OpenAI-style: {choices: [{message: {role, content, tool_calls}}]}
	const choices = outputParsed.choices;
	if (Array.isArray(choices) && choices.length > 0) {
		const choice = isRecord(choices[0]) ? choices[0] : null;
		const msg = choice && isRecord(choice.message) ? choice.message : null;
		if (!msg) return null;
		const toolCalls = normalizeOpenAiToolCalls(msg.tool_calls);
		if (!msg.content && !toolCalls) return null;
		return {
			role: asString(msg.role) || 'assistant',
			content: asString(msg.content) ?? '',
			toolCalls
		};
	}

	// Anthropic-style: {content: [{type: "text"|"tool_use", ...}]}
	if (Array.isArray(outputParsed.content)) {
		const message = normalizeMessage({
			role: asString(outputParsed.role) || 'assistant',
			content: outputParsed.content
		});
		if (message && (getMessageText(message.content) || message.toolCalls?.length)) return message;
	}

	return null;
}

export function parseInputMessages(input: string): ChatMessage[] | null {
	const inputParsed = tryParseJson(input);
	if (!inputParsed) return null;

	const inputMessages = isRecord(inputParsed)
		? inputParsed.messages
		: Array.isArray(inputParsed)
			? inputParsed
			: null;
	if (!Array.isArray(inputMessages)) return null;

	const messages: ChatMessage[] = [];
	const append = (raw: unknown) => {
		const message = normalizeMessage(raw);
		if (message) messages.push(message);
	};
	for (const msg of inputMessages) {
		if (!isRecord(msg) || msg.role !== 'user' || !Array.isArray(msg.content)) {
			append(msg);
			continue;
		}

		// Anthropic may return several tool results and a follow-up prompt in one user message.
		let start = 0;
		for (const [index, part] of msg.content.entries()) {
			if (!isRecord(part) || part.type !== 'tool_result') continue;
			if (index > start) append({ ...msg, content: msg.content.slice(start, index) });
			append({ role: 'tool', tool_call_id: part.tool_use_id, content: part.content ?? '' });
			start = index + 1;
		}
		if (start === 0) append(msg);
		else if (start < msg.content.length) append({ ...msg, content: msg.content.slice(start) });
	}
	return messages;
}

export function extractMessages(input: string, output: string): ChatMessage[] | null {
	const messages =
		parseInputMessages(input) ??
		(input.trim() ? [{ role: 'user', content: formatConversationContent(input) }] : []);

	const outputMessage =
		extractOutputMessage(output) ??
		(output.trim() ? { role: 'assistant', content: formatConversationContent(output) } : null);
	if (outputMessage) messages.push(outputMessage);

	return messages.length > 0 ? messages : null;
}

export function pairToolResults(messages: ChatMessage[]) {
	const pending = new Map<string, ToolCall>();
	const byCall = new Map<ToolCall, ChatMessage>();
	const pairedMessages = new Set<ChatMessage>();
	for (const message of messages) {
		for (const call of message.toolCalls ?? []) {
			if (call.id) pending.set(call.id, call);
		}
		if ((message.role === 'tool' || message.role === 'function') && message.toolCallId) {
			const call = pending.get(message.toolCallId);
			if (call) {
				byCall.set(call, message);
				pairedMessages.add(message);
				pending.delete(message.toolCallId);
			}
		}
	}
	return { byCall, pairedMessages };
}

function historyPrefixMatches(previous: ChatMessage[], current: ChatMessage[]): boolean {
	if (current.length < previous.length) return false;
	for (let i = 0; i < previous.length; i++) {
		if (JSON.stringify(previous[i]) !== JSON.stringify(current[i])) return false;
	}
	return true;
}

// buildConversationTimeline merges per-call payloads into one chat timeline.
// Most SDKs resend the full history each turn, so each turn's input is
// deduplicated against the previous turn's by complete-message prefix matching;
// when that fails (compacted histories, unparsable payloads) the caller
// should fall back to per-turn rendering (signaled by returning null).
export function buildConversationTimeline(turns: ConversationTurn[]): TimelineEntry[] | null {
	const entries: TimelineEntry[] = [];
	let previousInput: ChatMessage[] | null = null;

	for (const turn of turns) {
		const inputMessages = parseInputMessages(turn.input);
		if (!inputMessages) return null;

		let fresh = inputMessages;
		if (previousInput && previousInput.length > 0) {
			if (!historyPrefixMatches(previousInput, inputMessages)) return null;
			fresh = inputMessages.slice(previousInput.length);
		}
		for (const message of fresh) {
			entries.push({ kind: 'message', message });
		}

		const outputMessage = extractOutputMessage(turn.output);
		if (turn.output.trim() && !outputMessage) return null;
		if (outputMessage) {
			entries.push({ kind: 'message', message: outputMessage });
		}
		entries.push({ kind: 'turn-meta', turn });

		previousInput = inputMessages;
		if (outputMessage) {
			previousInput = [...inputMessages, outputMessage];
		}
	}

	return entries.length > 0 ? entries : null;
}

export function getMessageText(content: string | ContentPart[]): string {
	if (typeof content === 'string') return content;
	if (Array.isArray(content)) {
		return content
			.filter((p) => p.type === 'text' && p.text)
			.map((p) => p.text!)
			.join('\n');
	}
	return '';
}

export function getMessageImages(content: string | ContentPart[]): string[] {
	if (!Array.isArray(content)) return [];
	return content
		.filter((p) => p.type === 'image_url' && p.image_url?.url)
		.map((p) => p.image_url!.url);
}

export function getRoleLabel(role: string): string {
	switch (role) {
		case 'system':
			return 'System';
		case 'user':
			return 'User';
		case 'assistant':
			return 'Assistant';
		case 'tool':
			return 'Tool';
		case 'function':
			return 'Function';
		default:
			return role;
	}
}

export function formatConversationContent(raw: string): string {
	const parsed = tryParseJson(raw);
	if (parsed) {
		return JSON.stringify(parsed, null, 2);
	}
	return raw;
}
