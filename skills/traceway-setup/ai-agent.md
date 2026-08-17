# Instrumenting an AI Agent or Chatbot for Traceway

Read this file when "Analyze the Architecture" detected a conversational AI product: a chatbot, a multi-turn assistant, a tool-calling agent, or anything built on an agent framework. The base rules from the "AI Traces" section of SKILL.md still apply (one span per model call, and a `gen_ai.*` attribute promotes that span to an AI Trace). This file adds what conversational products need on top, so Traceway's Conversations and Users analytics light up: conversation grouping, per-user attribution, tool calls, and sub-agents.

## Before you write anything: three preconditions

Get these wrong and the export still returns 200 while the dashboard stays empty. Check all three first.

### 1. Your instrumentation must emit `gen_ai.*`

Traceway promotes a span to an AI Trace only when it carries an attribute literally prefixed `gen_ai.`. Agent-framework auto-instrumentation is split on this:

| Instrumentation | Emits | Works out of the box? |
|---|---|---|
| OpenLLMetry / Traceloop (`traceloop-sdk`, `@traceloop/node-server-sdk`) | `gen_ai.*` including `gen_ai.prompt` / `gen_ai.completion` | Yes |
| OpenLIT (`openlit`) | `gen_ai.*` | Yes |
| Vercel AI SDK `experimental_telemetry` | `gen_ai.*` for model and tokens, `ai.*` for content | Partly, see precondition 2 |
| OpenInference / Arize (`openinference-instrumentation-langchain`, `-llama-index`, `-crewai`, `-openai`) | `openinference.span.kind`, `llm.model_name`, `llm.token_count.*`, `input.value`, `output.value`, and **no `gen_ai.*`** | **No** |
| Langfuse / LangSmith OTel exporters | vendor-prefixed attributes | **No** |

An OpenInference-style span is not merely missing tokens. It is a root INTERNAL span that matches no promotion rule, so it is dropped outright: no AI trace, no span row, nothing on any page, while every OTLP export answers 200. If the project uses one of the "No" rows, pick one of these: switch to OpenLLMetry or OpenLIT, add a span processor that copies the vendor attributes onto their `gen_ai.*` equivalents, or instrument the model calls by hand with the skeleton below.

### 2. Content must land on the attribute names Traceway reads

Traceway reads the prompt from `gen_ai.prompt`, falling back to `trace.input`, then `span.input`. It reads the completion from `gen_ai.completion`, falling back to `trace.output`, then `span.output`. It does **not** read the newer OTel semconv names `gen_ai.input.messages` / `gen_ai.output.messages`, nor the Vercel AI SDK's `ai.prompt.messages` / `ai.response.text`.

Content on an unread name is stored as an inert attribute. The symptom is confusing: tokens and cost look perfectly healthy while the chat timeline is blank, tool calls are 0, and content flagging can never fire. If your SDK emits the semconv or `ai.*` names, copy them across with a span processor that runs before the exporting one:

```typescript
import type { ReadableSpan, SpanProcessor } from "@opentelemetry/sdk-trace-base";

class CopyGenAiContent implements SpanProcessor {
  onStart() {}
  onEnd(span: ReadableSpan) {
    const attrs = span.attributes as Record<string, unknown>;
    const copy = (target: string, ...sources: string[]) => {
      if (attrs[target] !== undefined) return;
      for (const source of sources) {
        const value = attrs[source];
        if (value === undefined) continue;
        attrs[target] = typeof value === "string" ? value : JSON.stringify(value);
        return;
      }
    };
    copy("gen_ai.prompt", "gen_ai.input.messages", "ai.prompt.messages");
    copy("gen_ai.completion", "gen_ai.output.messages", "ai.response.text");
  }
  async forceFlush() {}
  async shutdown() {}
}

// Order matters: this must come before the batch processor that exports.
new NodeSDK({
  spanProcessors: [new CopyGenAiContent(), new BatchSpanProcessor(exporter)],
});
```

### 3. `gen_ai.*` belongs on the model-call span and nowhere else

The `gen_ai.*` rule is the last one Traceway evaluates, so three earlier rules silently win the span:

- A root `SERVER` or `INTERNAL` span with any HTTP attribute (`http.route`, `http.request.method`, `url.path`) becomes an **Endpoint**. Never decorate the `/api/chat` route span with `gen_ai.conversation.id` or `user.id`, or the model call disappears from AI Traces.
- A `CONSUMER` span becomes a **Task**. An LLM call inside a queue worker needs its own child span, it cannot ride on the consumer span.
- An `INTERNAL` span carrying `exception.*` attributes is dropped entirely. The Issue is still recorded, the AI trace is not.

Use `SpanKind.CLIENT` for model calls, as the skeleton below does. That is not decoration, it is what keeps a failed call's AI trace alive.

## What you get when this is wired correctly

The AI Traces section of the dashboard has three tabs. Per-call spans alone only populate the first:

| Tab | Needs | Shows |
|---|---|---|
| **Traces** | any `gen_ai.*` span | per-agent call counts, latency, tokens, cost |
| **Conversations** | `gen_ai.conversation.id` for real grouping | turns, cost, tokens, tool calls per conversation; flagged-content badges; P95 outlier highlighting; tool/model/user filters |
| **Users** | `user.id` | per-customer analytics: conversation count, median/avg/min conversation length, cost per conversation, flagged conversations |

The Conversations tab is never literally empty. When `gen_ai.conversation.id` is absent Traceway falls back to the OTel trace id, so every HTTP request becomes its own one-turn "conversation". That is the failure mode to look for: a long list of UUID-named, one-turn rows rather than a blank page.

## Conversation identity

Traceway resolves a conversation id per call, in this order:

1. `gen_ai.conversation.id` span attribute. Set this explicitly, it is the reliable path.
2. `session.id` span or resource attribute.
3. The OTel trace id, stored in dashed-UUID form (trace `331b1aca5ef21647145c51e9dc39c601` becomes conversation `331b1aca-5ef2-1647-145c-51e9dc39c601`). Every AI span in one trace groups together, so a single agent run inside one request groups automatically, but turns arriving in later requests will NOT join it.

An empty-string `gen_ai.conversation.id` counts as absent and falls through to the next level, so a nullish chat id degrades quietly to per-request grouping instead of failing loudly.

Use the id your app already has for the chat session: the chat/thread id from your database, or the session id the frontend holds. Stable for the life of the conversation, different between conversations. Every model call in the agent loop (including retries and sub-agent calls) must carry the same value.

## `user.id`: what to set and why it matters

`user.id` is the customer dimension for the Users tab and the user filter on Conversations. Set it on every model-call span.

**Good values** are stable identifiers for the end user of YOUR product:

- your internal account/user id (best),
- the tenant or organization id for B2B products where the account is the unit you care about,
- an email address, if emails are acceptable in your telemetry.

**Bad values:**

- session ids, request ids, or anything random per conversation (that is `gen_ai.conversation.id`'s job; using it here makes every user look like a new user and destroys the per-customer medians),
- device ids that churn across reinstalls,
- leaving it unset (the Users tab stays empty and flagged conversations cannot be attributed).

**Privacy**: the value is stored on every AI trace row and is searchable and filterable by anyone with project access. If PII must not enter telemetry, use your internal id or a hash of it. The analytics only need stability, not readability.

## Tool calls

Traceway parses tool calls out of the resolved completion payload: `gen_ai.completion`, else `trace.output`, else `span.output`. The exact nesting is what gets matched, and a near miss silently yields zero tool calls. Exactly these three shapes work:

- **OpenAI chat completions**: `{"choices":[{"message":{"tool_calls":[{"function":{"name":"get_weather","arguments":"{...}"}}]}}]}`. The `choices[].message` wrapper is required. A bare `{"tool_calls":[...]}` parses to zero.
- **Anthropic messages**: `{"content":[{"type":"tool_use","name":"get_weather","input":{...}}]}`. The `content` object wrapper is required. A top-level `[{"type":"tool_use",...}]` array parses to zero.
- **OTel GenAI output messages**: `[{"role":"assistant","parts":[{"type":"tool_call","name":"get_weather"}]}]`. The `name` must sit on the part itself; nesting it under a `tool_call` object gives you a count with no name. This shape feeds tool counts and filters, but the chat timeline cannot render it, so prefer one of the two provider shapes if you want the conversation view.

The simplest correct choice is to serialize the provider's raw response: `JSON.stringify(response)` for Anthropic, `JSON.stringify({ choices: response.choices })` for OpenAI. Then tool names and counts appear on the conversation rows, become filterable, and render in the chat view with arguments and results.

Two rules for the tool executions themselves:

1. **Wrap each tool execution in a plain child span of the enclosing request or task span, not of the model-call span.** Name it `tool <name>` with attributes like `tool.name` and `tool.args`. Traceway re-roots every unpromoted child span onto its nearest promoted ancestor, and AI Traces have no waterfall view, so a tool span parented to the model call is stored and then rendered nowhere.

   ```
   POST /api/chat            SERVER   -> Endpoint (has a Spans tab)
   ├── chat gpt-4o           CLIENT   -> AI Trace (turn 1)
   ├── tool lookup_order     INTERNAL -> Span, shown on the Endpoint's Spans tab   OK
   └── db SELECT orders      CLIENT   -> Span, shown on the Endpoint's Spans tab   OK

   POST /api/chat            SERVER   -> Endpoint
   └── chat gpt-4o           CLIENT   -> AI Trace
       └── tool lookup_order INTERNAL -> re-rooted onto the AI Trace, rendered nowhere   BAD
   ```

   In practice: end the model-call span before you run the tools, or start the tool span from the request context instead of from inside `startActiveSpan` of the model call. The AI Trace still links back to its Endpoint through the shared distributed trace id either way.

2. **Do NOT put `gen_ai.*` attributes on tool-execution spans.** Any `gen_ai.*` attribute, including `gen_ai.operation.name = "execute_tool"`, promotes the span to its own AI Trace row. If it also carries the conversation id it becomes an extra *turn*, and if the model span already reported that tool in its completion payload the tool is counted twice: one model call plus one `execute_tool` span in the same conversation reports `turns: 2, toolCallCount: 2`. Use `execute_tool` plus `gen_ai.tool.name` only when you have no completion payload at all and accept one AI-trace row per tool invocation. Otherwise keep tool spans free of `gen_ai.*` and let the completion payload carry the names.

## Sub-agents

When the main agent delegates to sub-agents (a researcher, a planner, a summarizer), give each sub-agent's model calls their own `trace.name` ("Research Sub-Agent") while keeping the SAME `gen_ai.conversation.id` and `user.id`. Each agent then appears separately on the Traces tab with its own latency and cost profile, while the Conversations tab aggregates the whole delegation as one conversation: turns ordered by time, models and tool names unioned across agents.

The single merged chat timeline needs one more thing: each turn's `gen_ai.prompt` must be a superset of the previous turn's message list, with the same role order. A sub-agent that sends its own system prompt or a compacted history breaks that match, and the conversation page falls back to rendering each turn as its own input/output block. That is cosmetic. Turn counts, costs and tool names are unaffected.

## Putting it together (agent loop skeleton)

```typescript
import { SpanKind, trace } from "@opentelemetry/api";

const tracer = trace.getTracer("agent");
const MODEL = "gpt-4o";

// Traceway has no server-side price list. You compute cost, in USD per token.
const PRICES: Record<string, { in: number; out: number }> = {
  "gpt-4o": { in: 2.5 / 1_000_000, out: 10 / 1_000_000 },
};

async function agentTurn(conversationId: string, userId: string, messages: Message[]) {
  // CLIENT, not INTERNAL: an INTERNAL span carrying exception.* attributes is
  // dropped, so a failed call would lose its AI trace.
  const span = tracer.startSpan(`chat ${MODEL}`, { kind: SpanKind.CLIENT });
  try {
    const response = await llm.chat({ model: MODEL, messages, tools });
    const usage = response.usage;
    const price = PRICES[MODEL] ?? { in: 0, out: 0 };
    span.setAttributes({
      "trace.name": "Support Chat Agent",          // sub-agents use their own name here
      "gen_ai.conversation.id": conversationId,     // same for every call in the conversation
      "user.id": userId,                            // stable end-customer id
      "gen_ai.system": "openai",
      "gen_ai.operation.name": "chat",
      "gen_ai.request.model": MODEL,
      "gen_ai.response.model": response.model,
      "gen_ai.usage.input_tokens": usage.prompt_tokens,       // integers, never strings
      "gen_ai.usage.output_tokens": usage.completion_tokens,
      "gen_ai.usage.input_cost": usage.prompt_tokens * price.in,        // doubles
      "gen_ai.usage.output_cost": usage.completion_tokens * price.out,
      "gen_ai.response.finish_reason": response.choices[0].finish_reason,
      "gen_ai.prompt": JSON.stringify({ messages }),
      "gen_ai.completion": JSON.stringify({ choices: response.choices }),
    });
    return response;
  } finally {
    // End before running the tools, so tool spans attach to the request span
    // rather than to this one.
    span.end();
  }
}
```

Each iteration of the tool-call loop is its own span and call. That is what "turns" counts. Resending the full message history in `gen_ai.prompt` each turn is expected, the conversation view deduplicates it by role-sequence prefix. Include the assistant tool-call message and the `role: "tool"` result message in the resent history, because that is where the chat view gets tool results from.

**Cost is yours to compute.** Traceway stores whatever you send and has no model price list. Without `gen_ai.usage.input_cost` / `.output_cost` (or `.total_cost` directly) every cost figure stays at 0.00: the Conversations cost column, the Users cost-per-conversation and total cost, the P95 outlier highlighting, and the `ai_trace_cost` / `ai_conversation_cost` notification rules. Keep a small per-model price map in the app and multiply by the token counts. Send them as numeric OTLP attributes, integers for tokens and doubles for costs. A number sent as a JSON string is silently dropped and stored as 0, with no error on the export.

## What lights up what

When a dashboard column is blank, work backwards from here.

| Dashboard column | Attribute that fills it | Blank means |
|---|---|---|
| Traces tab grouping name | `trace.name`, else the span name | never blank |
| Model / Response model | `gen_ai.request.model` / `gen_ai.response.model` | attribute unset |
| Provider | `gen_ai.system`, else `gen_ai.provider.name` | neither set |
| Tokens | `gen_ai.usage.input_tokens` / `.output_tokens` (integers only); `total_tokens` defaults to their sum | unset, or sent as a string |
| Cached / Reasoning tokens | `gen_ai.usage.input_tokens.cached` / `gen_ai.usage.output_tokens.reasoning` | unset |
| Cost | `gen_ai.usage.input_cost` / `.output_cost` (doubles); `total_cost` defaults to their sum | you did not compute pricing |
| Finish reason | `gen_ai.response.finish_reason`, else `gen_ai.response.finish_reasons` (array, joined with commas) | unset |
| Conversation id | `gen_ai.conversation.id`, then `session.id` (span), then `session.id` (resource), then the trace id as a UUID | never blank |
| User | `user.id` | unset, and the Users tab stays empty |
| Tool count / names | tool calls parsed from `gen_ai.completion`, then `trace.output`, then `span.output`; else `gen_ai.operation.name=execute_tool` plus `gen_ai.tool.name` | payload not in one of the three shapes above |
| Chat timeline | `gen_ai.prompt`, then `trace.input`, then `span.input`, plus the completion chain above. The prompt must parse to `{"messages":[...]}` or to a bare array of messages | content on attribute names Traceway does not read, or a prompt in some other JSON shape |
| Flagged badge | matched at ingest against the project's packs and custom terms, over that same resolved input/output | no content stored, or no term matched |
| Everything else you set | kept in the trace's attribute bag, minus the standard keys above | n/a |

## OpenRouter zero-code path

If the app calls OpenRouter and uses the Broadcast/Observability feature instead of its own spans, the same analytics work through request-level fields:

- Pass `user` in the request body (up to 128 chars), the end-customer id, same guidance as `user.id` above. Without it, calls attribute to the OpenRouter account itself, which makes the Users tab useless.
- Pass `session_id` in the request body (up to 256 chars, also accepted as the `x-session-id` header) for conversation grouping. OpenRouter exports it as the OTLP `session.id` attribute, which is Traceway's second-level conversation key. If the app also emits its own spans, use the SAME value for `gen_ai.conversation.id` there, or one chat splits into two conversations.
- Optional `trace` metadata names the agent: `trace_name` for the root, `generation_name` for the individual call.

Full reference: https://docs.tracewayapp.com/client/openrouter

## Content flagging (no code required)

Prompt and completion text is scanned at ingest against built-in profanity packs (en/de/es/fr/it/pt/sr) plus per-project custom terms. Nothing to instrument, it works as soon as `gen_ai.prompt` / `gen_ai.completion` are populated (or the `trace.input` / `trace.output` fallbacks). Configure it from the Conversations page's **Flagged terms** button, which opens the project settings AI tab and needs write access on the project.

Four things to tell the user:

- **Only `en` is on by default, and the pack list replaces it.** Selecting `es` turns English off. Select every language your users actually write in. Deselecting all leaves custom terms only.
- **Flags are computed at ingest and never backfilled.** Adding a term affects future calls only, so set custom terms up front.
- **Matching is whole-token and case-insensitive.** `acmecorp` matches "AcmeCorp" and "AcmeCorp-adjacent" but not "AcmeCorporation". Multi-word terms work as phrases. Terms are stored lowercased.
- **Limits:** 200 custom terms, 100 characters each, at most 20 distinct terms reported per call, and only the first 256 KB of each text is scanned.

Custom terms (competitor names, refund and cancellation phrases, compliance triggers) are usually worth setting up on day one.

## Payload size and retention

Each call's resolved input and output are written as one blob per AI trace to object storage (S3, or `STORAGE_PATH` on disk) under `ai-traces/<projectId>/<traceId>.json`. The row itself holds only the metrics. Because agents resend the whole history every turn, a 30-turn conversation stores roughly 30 copies of that history. Budget for it, and consider trimming long system prompts or retrieved context out of `gen_ai.prompt` if volume matters more than fidelity.

Read-side limits worth knowing: a conversation renders at most 1000 turns, and only the first 200 of them get their stored prompt and completion blobs attached, so beyond that the turns still show metrics but no chat body. AI trace rows age out with the project's telemetry retention (`SQLITE_RETENTION_DAYS` / `DUCKDB_RETENTION_DAYS`, default 30 days; no TTL on ClickHouse), while the payload blobs are not on that cleanup path.

## What this unlocks beyond the dashboard

Once conversation ids, costs and content are flowing, three event-driven notification rule types become usable (Settings, then Notification rules):

- **`ai_trace_cost`** fires when a single call exceeds a cost threshold. Needs `gen_ai.usage.*_cost`.
- **`ai_conversation_cost`** fires when one conversation's rolling 24h cost crosses a threshold. Needs `gen_ai.conversation.id` and the cost attributes. Without a real conversation id the trace-id fallback makes this meaningless, since every request looks like a fresh conversation.
- **`ai_flagged_content`** fires on a flagged-term match, optionally filtered to specific terms. Needs the content attributes populated and the terms configured.

Mention these when you finish the integration. A runaway-cost alert on a customer-facing agent is usually worth setting up the same day.

## Verify

After wiring, run one real multi-turn chat with a tool call, then check in the dashboard (or via the API):

1. **AI Traces, Traces tab**: your `trace.name` appears with the right call count. Nothing here at all means no span carries a `gen_ai.*` attribute. Re-check precondition 1, and check you did not hang the attributes on the HTTP route span.
2. **AI Traces, Conversations tab**: exactly ONE row for the chat, turn count matching the number of model calls, tool names listed, non-zero cost. A list of UUID-named one-turn rows means `gen_ai.conversation.id` is missing and Traceway fell back to the trace id. Cost 0.00 means no `gen_ai.usage.*_cost` attributes. Blank tool columns mean the completion payload is not in one of the three exact shapes above.
3. **AI Traces, Users tab**: the test user appears with 1 conversation. Empty means `user.id` is unset.
4. Open the conversation: the chat timeline renders with tool calls inline, and each turn links to its call. Turns present but the chat body blank means the content landed on attribute names Traceway does not read (`gen_ai.input.messages`, `ai.prompt.messages`, and so on). Confirm with the Raw JSON toggle on that page.
5. Open the Endpoint that wrapped the chat request: the tool-execution spans appear on its Spans tab. Missing means they were parented to the model-call span instead of the request span.
