# Instrumenting an AI Agent or Chatbot for Traceway

Read this file when the analysis detected a conversational AI product: a chatbot, a multi-turn assistant, a tool-calling agent, or anything built on an agent framework. The base procedure is `ai-usage.md` in this skill directory (inventory the call sites, one OpenTelemetry GenAI span per model call, the attributes Traceway reads, verification). This file adds what conversational products need on top, so Traceway's Conversations and Users analytics light up: conversation grouping, per-user attribution, tool calls, and sub-agents.

## Before you write anything: three preconditions

Get these wrong and the export still returns 200 while the dashboard stays empty. Check all three first.

### 1. Your instrumentation must emit `gen_ai.*`

Traceway promotes a span to an AI Trace only when it carries an attribute literally prefixed `gen_ai.`. Agent-framework auto-instrumentation is split on this:

| Instrumentation | Emits | Promoted to an AI Trace? |
|---|---|---|
| OpenLLMetry / Traceloop (`traceloop-sdk`, `@traceloop/node-server-sdk`) | `gen_ai.*` including `gen_ai.prompt` / `gen_ai.completion` | Yes |
| OpenLIT (`openlit`) | `gen_ai.*` | Yes |
| Vercel AI SDK `experimental_telemetry` | `gen_ai.*` for model and tokens, `ai.*` for content | Partly, see precondition 2 |
| OpenInference / Arize (`openinference-instrumentation-langchain`, `-llama-index`, `-crewai`, `-openai`) | `openinference.span.kind`, `llm.model_name`, `llm.token_count.*`, `input.value`, `output.value`, and **no `gen_ai.*`** | **No** |
| Langfuse / LangSmith OTel exporters | vendor-prefixed attributes | **No** |

An OpenInference-style span is not merely missing tokens. It matches no promotion rule, so it is stored as a plain span and nothing else: no AI Trace, no tokens, no conversation, while every OTLP export answers 200. If the project uses one of the "No" rows, pick one of these: switch to OpenLLMetry or OpenLIT, add a span processor that copies the vendor attributes onto their `gen_ai.*` equivalents, or instrument the model calls by hand with the skeleton below.

"Yes" only means the span is promoted. Content, cost, the conversation id and the user id still have to be on the attributes Traceway reads, so check one exported span against "Self-check a span" in `ai-usage.md` whichever library produced it.

### 2. Content must land on the attribute names Traceway reads

Traceway reads the prompt from `gen_ai.prompt`, falling back to `trace.input`, then `span.input`. It reads the completion from `gen_ai.completion`, falling back to `trace.output`, then `span.output`. It does **not** read the newer OTel semconv names `gen_ai.input.messages` / `gen_ai.output.messages`, nor the Vercel AI SDK's `ai.prompt.messages` / `ai.response.text`.

With content capture enabled, format the captured messages in a library hook or a Traceway-specific export transformation. A direct copy of the conventions’ `parts` arrays leaves the chat view without renderable messages. Map text parts to `content`, tool requests to assistant `tool_calls`, and tool responses to `role: "tool"` messages with the matching `tool_call_id`. Wrap output in one of the provider shapes under “Tool calls” below. With capture off, omit prompts, completions, tool arguments and results from the Traceway export, including vendor-specific keys and events.

### 3. `gen_ai.*` belongs on the model-call span and nowhere else

The `gen_ai.*` rule is the last one Traceway evaluates, so three earlier rules silently win the span:

- A root `SERVER` or `INTERNAL` span with any HTTP attribute (`http.route`, `http.request.method`, `url.path`) becomes an **Endpoint**. Never decorate the `/api/chat` route span with `gen_ai.conversation.id` or `user.id`, or the model call disappears from AI Traces.
- A `CONSUMER` span becomes a **Task**. An LLM call inside a queue worker needs its own child span, it cannot ride on the consumer span.
- An `INTERNAL` span carrying `exception.*` attributes is never promoted. The Issue is recorded and the span is stored, but there is no AI Trace.

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

1. **Wrap each tool execution in a plain span, started after the model-call span has ended.** Name it `execute_tool <name>` (the GenAI conventions' name for it) with `tool.name` and `tool.call.id`. Arguments and results (`tool.args`, `tool.result`) are content, so add them only when content capture is on. Because the model call is over by then, the tool span becomes a child of the enclosing request or task span and sits next to the model calls in that Endpoint's or Task's waterfall, in the order things happened:

   ```
   POST /api/chat                 SERVER   -> Endpoint
   ├── chat gpt-4o                CLIENT   -> AI Trace (turn 1, asks for the tool)
   ├── execute_tool lookup_order  INTERNAL -> plain span
   │   └── SELECT orders          CLIENT   -> plain span
   └── chat gpt-4o                CLIENT   -> AI Trace (turn 2, answers)
   ```

   A span started while the model-call span is still active becomes its child instead. It is then shown on that AI Trace's own Spans card as well, which is the right place for the provider HTTP request and wrong for a tool that ran afterwards. Each AI Trace also links back to its Endpoint through the shared trace id.

2. **Do NOT put `gen_ai.*` attributes on tool-execution spans.** Any `gen_ai.*` attribute, including `gen_ai.operation.name = "execute_tool"`, promotes the span to its own AI Trace row. If it also carries the conversation id it becomes an extra *turn*, and if the model span already reported that tool in its completion payload the tool is counted twice: one model call plus one `execute_tool` span in the same conversation reports `turns: 2, toolCallCount: 2`. Use `execute_tool` plus `gen_ai.tool.name` only when you have no completion payload at all and accept one AI-trace row per tool invocation. Otherwise keep tool spans free of `gen_ai.*` and let the completion payload carry the names.

## Side calls

Some calls serve a conversation without being a turn of it: a title for the chat, a summary, a moderation check, an embedding for retrieval. Give them their own `trace.name` ("Chat Title"), and choose deliberately whether they carry the conversation's id:

- **With the id** (the default): the call's cost counts toward the conversation, which is what the `ai_conversation_cost` alert and the cost columns need. It also counts as a turn, and because its prompt is not the running history, the conversation page shows every turn as its own block instead of one merged chat.
- **Without the id**: Traceway falls back to the span or resource `session.id`, then to the trace id. Calls with the same fallback id still group together. To guarantee separate attribution, supply a distinct stable conversation id for the side work.

## Sub-agents

When the main agent delegates to sub-agents (a researcher, a planner, a summarizer), give each sub-agent's model calls their own `trace.name` ("Research Sub-Agent") while keeping the SAME `gen_ai.conversation.id` and `user.id`. Each agent then appears separately on the Traces tab with its own latency and cost profile, while the Conversations tab aggregates the whole delegation as one conversation: turns ordered by time, models and tool names unioned across agents.

The single merged chat timeline needs one more thing: each turn’s input must start with the complete previous input and its assistant response, in order. A sub-agent that sends its own system prompt or a compacted history breaks that match, and the conversation page falls back to rendering each turn as its own input/output block. That is cosmetic. Turn counts, costs and tool names are unaffected.

## Putting it together (agent loop skeleton)

`tracedChat` is the helper from Step D of `ai-usage.md`: it starts the model-call span, sets the request-side attributes before the request so a failed call keeps them, records tokens, cost, content and errors, and ends the span. A conversation adds three things to it: the same `conversationId` and `userId` on every call, the full history in every prompt, and tool spans between the calls.

```typescript
import { SpanKind, trace } from "@opentelemetry/api";

const tracer = trace.getTracer("agent");

async function answer(conversationId: string, userId: string, history: Message[], userMessage: string) {
  // Same three values on every call of the conversation. A sub-agent changes only `feature`.
  const call = { feature: "Support Chat Agent", conversationId, userId };
  history.push({ role: "user", content: userMessage });

  for (let step = 0; step < MAX_STEPS; step++) {
    // One iteration = one provider request = one span = one turn.
    const response = await tracedChat(openai, call, { model: MODEL, messages: history, tools });
    const message = response.choices[0].message;
    history.push(message);                       // the assistant message, tool calls included
    if (!message.tool_calls?.length) return message.content;

    // The model-call span has ended, so these become its siblings under the request span.
    for (const toolCall of message.tool_calls) {
      if (toolCall.type !== "function") throw new Error("Expected a function tool call");
      const { name, arguments: args } = toolCall.function;
      await tracer.startActiveSpan(`execute_tool ${name}`, { kind: SpanKind.INTERNAL }, async (span) => {
        try {
          span.setAttributes({ "tool.name": name, "tool.call.id": toolCall.id });
          const result = await runTool(name, JSON.parse(args));
          history.push({ role: "tool", tool_call_id: toolCall.id, content: JSON.stringify(result) });
        } finally {
          span.end();
        }
      });
    }
  }
  throw new Error("Agent step limit reached");
}
```

Each iteration of the tool-call loop is its own span and call. That is what "turns" counts. Resending the full message history in `gen_ai.prompt` each turn is expected, the conversation view deduplicates matching messages at the start of the history. Include the assistant tool-call message and the `role: "tool"` result message in the resent history, because that is where the chat view gets tool results from.

**Cost is yours to compute.** Traceway stores whatever you send and has no model price list. Without `gen_ai.usage.input_cost` / `.output_cost` (or `.total_cost` directly) every cost figure stays at 0.00: the Conversations cost column, the Users cost-per-conversation and total cost, the P95 outlier highlighting, and the `ai_trace_cost` / `ai_conversation_cost` notification rules. Keep a small per-model price map in the app and multiply by the token counts. Send them as numeric OTLP attributes, integers for tokens and doubles for costs. A number sent as a JSON string is silently dropped and stored as 0, with no error on the export.

## What lights up what

When a dashboard column is blank, work backwards from here.

| Dashboard column | Attribute that fills it | Blank means |
|---|---|---|
| Traces tab grouping name | `trace.name`, else the span name | never blank |
| Model / Response model | `gen_ai.request.model` / `gen_ai.response.model` | attribute unset |
| Provider | `gen_ai.provider.name` (the older `gen_ai.system` wins when both are set) | neither set |
| Tokens | `gen_ai.usage.input_tokens` / `.output_tokens` (integers only); `total_tokens` defaults to their sum | unset, or sent as a string |
| Cached / Reasoning tokens | `gen_ai.usage.cache_read.input_tokens` / `gen_ai.usage.reasoning.output_tokens` (`gen_ai.usage.input_tokens.cached` / `gen_ai.usage.output_tokens.reasoning` are read too) | unset, or sent as a string |
| Cost | `gen_ai.usage.input_cost` / `.output_cost` (doubles); `total_cost` defaults to their sum | you did not compute pricing |
| Finish reason | `gen_ai.response.finish_reasons` (array, joined with commas); a scalar `gen_ai.response.finish_reason` wins when both are set | unset |
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

Use the test environment agreed in Step E of `ai-usage.md` for a multi-turn chat with a tool call. If it was exported, check the dashboard. These pages need a dashboard login: an agent holding only the ingest token finishes with "Self-check a span" in `ai-usage.md` and hands these checks to the user, with the conversation id and user id of the test chat.

1. **AI Traces, Traces tab**: your `trace.name` appears with the right call count. Nothing here at all means no span carries a `gen_ai.*` attribute. Re-check precondition 1, and check you did not hang the attributes on the HTTP route span.
2. **AI Traces, Conversations tab**: exactly ONE row for the chat, turn count matching the number of model calls, expected cost, and tool names when completions were captured. A list of UUID-named one-turn rows means `gen_ai.conversation.id` is missing and Traceway fell back to the trace id. For a billable test call, check the numeric cost attributes if the cost is unexpectedly zero. Blank tool columns mean the completion payload is not in one of the three exact shapes above.
3. **AI Traces, Users tab**: the test user appears with 1 conversation. Empty means `user.id` is unset.
4. Open the conversation: the chat timeline renders with tool calls inline, and each turn links to its call. Turns present but the chat body blank means the content landed on attribute names Traceway does not read (`gen_ai.input.messages`, `ai.prompt.messages`, and so on). Confirm with the Raw JSON toggle on that page.
5. Open the Endpoint that wrapped the chat request: the model calls and the tool-execution spans appear in its waterfall in the order they ran. A tool span nested under a model call was started before that model-call span ended. A tool listed as its own row on the Traces tab carries a `gen_ai.*` attribute.
