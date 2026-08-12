# Instrumenting an AI Agent or Chatbot for Traceway

Read this file when Step 1 detected a conversational AI product: a chatbot, a multi-turn assistant, a tool-calling agent, or anything built on an agent framework. The base rules from SKILL.md Step 4 still apply (one span per model call, `gen_ai.*` attributes promote the span to an AI Trace). This file adds what conversational products need on top, so Traceway's Conversations and Users analytics light up: conversation grouping, per-user attribution, tool calls, and sub-agents.

## What you get when this is wired correctly

The AI Traces section of the dashboard has three tabs. Per-call spans alone only populate the first:

| Tab | Needs | Shows |
|---|---|---|
| **Traces** | any `gen_ai.*` span | per-agent call counts, latency, tokens, cost |
| **Conversations** | `gen_ai.conversation.id` | turns, cost, tokens, tool calls per conversation; flagged-content badges; P95 outlier highlighting; tool/model/user filters |
| **Users** | `gen_ai.conversation.id` + `user.id` | per-customer analytics: conversation count, median/avg/min conversation length, cost per conversation, flagged conversations |

## Conversation identity

Traceway resolves a conversation id per call, in this order:

1. `gen_ai.conversation.id` span attribute — set this explicitly; it is the reliable path.
2. `session.id` span or resource attribute.
3. The OTel trace id — a single agent run inside one request groups automatically, but turns arriving in later requests will NOT join it.

Use the id your app already has for the chat session: the chat/thread id from your database, or the session id the frontend holds. Stable for the life of the conversation, different between conversations. Every model call in the agent loop (including retries and sub-agent calls) must carry the same value.

## `user.id`: what to set and why it matters

`user.id` is the customer dimension for the Users tab and the user filter on Conversations. Set it on every model-call span.

**Good values** — a stable identifier for the end user of YOUR product:

- your internal account/user id (best),
- the tenant or organization id for B2B products where the account is the unit you care about,
- an email address, if emails are acceptable in your telemetry.

**Bad values:**

- session ids, request ids, or anything random per conversation — that is `gen_ai.conversation.id`'s job; using it here makes every user look like a new user and destroys the per-customer medians,
- device ids that churn across reinstalls,
- leaving it unset — the Users tab stays empty and flagged conversations cannot be attributed.

**Privacy**: the value is stored on every AI trace row and is searchable and filterable by anyone with project access. If PII must not enter telemetry, use your internal id or a hash of it — the analytics only need stability, not readability.

## Tool calls

Traceway parses tool calls out of the completion payload (`gen_ai.completion`), so the shape matters. Any of these work:

- OpenAI chat completions: `{"choices":[{"message":{"tool_calls":[{"function":{"name":...,"arguments":...}}]}}]}`
- Anthropic messages: `{"content":[{"type":"tool_use","name":...,"input":...}]}`
- OTel GenAI output messages: `[{"role":"assistant","parts":[{"type":"tool_call","name":...}]}]`

Set `gen_ai.completion` to the raw provider response (or the shapes above) and tool names/counts appear on the conversation rows, become filterable, and render in the chat view with arguments and results.

Two rules for the tool executions themselves:

1. **Wrap each tool execution in a plain child span** (name it `tool <name>`, attributes like `tool.name`, `tool.args`) so it shows in the trace waterfall.
2. **Do NOT put `gen_ai.*` attributes on tool-execution spans.** Any `gen_ai.*` attribute promotes the span to its own AI Trace row, which inflates the conversation's turn count. The single exception is a span that represents a standalone tool-execution step in the OTel semconv style (`gen_ai.operation.name = "execute_tool"` + `gen_ai.tool.name`) — Traceway counts that as a tool call, but the completion-payload path above is preferred.

## Sub-agents

When the main agent delegates to sub-agents (a researcher, a planner, a summarizer), give each sub-agent's model calls their own `trace.name` ("Research Sub-Agent") while keeping the SAME `gen_ai.conversation.id` and `user.id`. Each agent then appears separately on the Traces tab with its own latency/cost profile, while the Conversations tab shows the whole delegation as one conversation, with the sub-agent turns interleaved in the chat timeline.

## Putting it together (agent loop skeleton)

```typescript
async function agentTurn(conversationId: string, userId: string, messages: Message[]) {
  return tracer.startActiveSpan(`chat ${MODEL}`, { kind: SpanKind.CLIENT }, async (span) => {
    const response = await llm.chat({ model: MODEL, messages, tools });
    span.setAttributes({
      "trace.name": "Support Chat Agent",           // sub-agents use their own name here
      "gen_ai.conversation.id": conversationId,      // same for every call in the conversation
      "user.id": userId,                             // stable end-customer id
      "gen_ai.system": "openai",
      "gen_ai.request.model": MODEL,
      "gen_ai.response.model": response.model,
      "gen_ai.usage.input_tokens": response.usage.prompt_tokens,
      "gen_ai.usage.output_tokens": response.usage.completion_tokens,
      "gen_ai.response.finish_reason": response.choices[0].finish_reason,
      "gen_ai.prompt": JSON.stringify({ messages }),
      "gen_ai.completion": JSON.stringify({ choices: response.choices }),
    });
    span.end();
    return response;
  });
}
```

Each iteration of the tool-call loop is its own span/call — that is what "turns" counts. Resending the full message history in `gen_ai.prompt` each turn is expected; the conversation view deduplicates it.

## OpenRouter zero-code path

If the app calls OpenRouter and uses the Broadcast/Observability feature instead of its own spans, the same analytics work through request-level fields:

- Pass `user` in the request body (up to 128 chars) — the end-customer id, same guidance as `user.id` above. Without it, calls attribute to the OpenRouter account itself, which makes the Users tab useless.
- Pass `session_id` in the request body (or the `x-session-id` header) for conversation grouping.
- Optional `trace` metadata (`trace_name`) names the agent.

## Content flagging (no code required)

Prompt and completion text is scanned at ingestion against per-project profanity language packs (en/de/es/fr/it/pt/sr) plus custom terms. Configure from the Conversations page's **Flagged terms** button. Nothing to instrument — it works as soon as `gen_ai.prompt`/`gen_ai.completion` are populated. Mention this to the user; custom terms (competitor names, compliance phrases) are usually worth setting up on day one.

## Verify

After wiring, run one real multi-turn chat with a tool call, then check in the dashboard (or via the API):

1. **AI Traces -> Conversations**: the conversation appears, turn count matches the number of model calls, tool names are listed.
2. **AI Traces -> Users**: the test user appears with 1 conversation.
3. Open the conversation: the chat timeline renders with tool calls inline; each turn links to its call.
4. If any of these are empty, the corresponding attribute is missing — conversation id (tab 1 empty), user id (tab 2 empty), or completion payload shape (tool columns empty).
