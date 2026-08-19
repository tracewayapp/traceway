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

function normalizeInputMessage(value: unknown): ChatMessage | null {
	if (!isRecord(value)) return null;
	const role = asString(value.role);
	if (!role) return null;

	// Anthropic-style content blocks may carry tool_use / tool_result parts.
	if (Array.isArray(value.content)) {
		const toolCalls: ToolCall[] = [];
		let toolCallId: string | undefined;
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
			} else if (type === 'tool_result') {
				toolCallId = asString(item.tool_use_id);
				const inner = item.content;
				if (typeof inner === 'string') {
					parts.push({ type: 'text', text: inner });
				} else if (Array.isArray(inner)) {
					for (const innerValue of inner) {
						if (isRecord(innerValue) && innerValue.type === 'text') {
							const text = asString(innerValue.text);
							if (text) parts.push({ type: 'text', text });
						}
					}
				}
			} else if (type) {
				const image = isRecord(item.image_url) ? asString(item.image_url.url) : undefined;
				parts.push({
					type,
					text: asString(item.text),
					image_url: image ? { url: image } : undefined
				});
			}
		}
		const message: ChatMessage = {
			role: toolCallId ? 'tool' : role,
			content: parts
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
	if (!isRecord(outputParsed)) return null;

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
		const blocks = outputParsed.content.filter(isRecord);
		const text = blocks
			.filter((block) => block.type === 'text')
			.map((block) => asString(block.text) ?? '')
			.join('');
		const toolCalls: ToolCall[] = blocks
			.filter((block) => block.type === 'tool_use' && asString(block.name))
			.map((block) => ({
				id: asString(block.id),
				name: asString(block.name)!,
				arguments: JSON.stringify(block.input ?? {})
			}));
		if (!text && toolCalls.length === 0) return null;
		return {
			role: asString(outputParsed.role) || 'assistant',
			content: text,
			toolCalls: toolCalls.length > 0 ? toolCalls : undefined
		};
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
	for (const msg of inputMessages) {
		const normalized = normalizeInputMessage(msg);
		if (normalized) messages.push(normalized);
	}
	return messages;
}

export function extractMessages(input: string, output: string): ChatMessage[] | null {
	const messages = parseInputMessages(input);
	if (!messages) return null;

	const outputMessage = extractOutputMessage(output);
	if (outputMessage) messages.push(outputMessage);

	return messages.length > 0 ? messages : null;
}

// pairToolResults maps a tool call id to the tool message that carries its
// result, so ToolCallBlock can render call and result together. Paired tool
// messages should be hidden from the main message list.
export function pairToolResults(messages: ChatMessage[]): Map<string, ChatMessage> {
	const paired = new Map<string, ChatMessage>();
	for (const msg of messages) {
		if (
			(msg.role === 'tool' || msg.role === 'function') &&
			msg.toolCallId &&
			!paired.has(msg.toolCallId)
		) {
			paired.set(msg.toolCallId, msg);
		}
	}
	return paired;
}

function roleSequenceMatches(previous: ChatMessage[], current: ChatMessage[]): boolean {
	if (current.length < previous.length) return false;
	for (let i = 0; i < previous.length; i++) {
		if (previous[i].role !== current[i].role) return false;
	}
	return true;
}

// buildConversationTimeline merges per-call payloads into one chat timeline.
// Most SDKs resend the full history each turn, so each turn's input is
// deduplicated against the previous turn's by role-sequence prefix matching;
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
			if (!roleSequenceMatches(previousInput, inputMessages)) return null;
			fresh = inputMessages.slice(previousInput.length);
		}
		for (const message of fresh) {
			entries.push({ kind: 'message', message });
		}

		const outputMessage = extractOutputMessage(turn.output);
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
