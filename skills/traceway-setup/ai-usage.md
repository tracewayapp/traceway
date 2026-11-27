# Tracking AI / LLM Usage in Traceway

Read this file for a scoped run that names AI usage (`/traceway-setup track ai use`, "instrument the LLM calls", "track token cost", "monitor the agent"), and for Step 6 of a full setup. It is a complete procedure: find every model call in the codebase, instrument each one as an OpenTelemetry GenAI span, and confirm it lands in Traceway's AI Traces.

The instrumentation follows the [OpenTelemetry GenAI semantic conventions](https://github.com/open-telemetry/semantic-conventions-genai) (status: Development, so names still move between releases). Traceway reads most of that spec as written and differs in a few places. Those places are listed in "Where Traceway differs from the spec" and they are the reason a spec-perfect integration can still show blank chat views and 0.00 costs.

## What done looks like

- Each model call appears once under its feature or agent on `/ai-traces`, with duration, model, provider and available usage and cost.
- A failed call is still a row, marked as an error, with an Issue behind it.
- When the user allowed content capture, the call's page shows the prompt and completion as a chat, with tool calls.
- For a chatbot, assistant or agent: one row per conversation on the Conversations tab and one per customer on the Users tab. That part is in [ai-agent.md](ai-agent.md).

## Step A: Find where telemetry goes

AI Traces are OTLP spans, so they need a tracer provider that exports to Traceway. Inspect the existing export path first. The table describes what to propose after you have found a backend model call in scope. If the inventory is empty, report it and stop before requesting credentials or configuring an exporter.

1. Search for an existing OpenTelemetry setup: a tracer provider or SDK init (`NodeSDK`, `TracerProvider`, `opentelemetry-instrument`, the Java agent, `otel.SetTracerProvider`), `OTEL_EXPORTER_OTLP_*` variables in env files and deploy config, and the endpoint they point at. Report variable names only, never values.
2. Decide which case applies:

| Found | Do |
|---|---|
| Traces already export to Traceway (`<instance>/api/otel`) | Build on it. Use the global tracer; add no second provider and no second exporter. |
| Traces export to another backend or a collector | Check the collector’s trace pipeline for an existing Traceway destination. If Traceway is new, confirm whether it replaces or supplements the current destination. A collector can fan out the traces, or the existing provider can use a second span processor. |
| No trace export for this component | Reuse any existing SDK initialization and propose the missing pieces for model calls: the language's OTel SDK, an OTLP/HTTP trace exporter and a resource with `service.name` and `service.version`. In a scoped run stop there: do not add HTTP, database or log instrumentation, and say in the final report what is and is not traced. |

   For a project with no existing provider, configure one from the variables the repository already uses for the Traceway URL and token. Do not copy the token into a second variable such as `OTEL_EXPORTER_OTLP_HEADERS` just to use the standard names; when you do configure through `OTEL_EXPORTER_OTLP_*` variables, read "Exporter configuration" in SKILL.md Step 4 first (`OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf` above all).

   ```python
   # pip install opentelemetry-sdk opentelemetry-exporter-otlp-proto-http   (and add both to requirements)
   import os
   from opentelemetry import trace
   from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
   from opentelemetry.sdk.resources import Resource
   from opentelemetry.sdk.trace import TracerProvider
   from opentelemetry.sdk.trace.export import BatchSpanProcessor

   # service.version: the package version or the deploy's release variable, never a constant that goes stale.
   resource = Resource.create({"service.name": "docs-assistant", "service.version": os.getenv("APP_VERSION", "dev")})
   provider = TracerProvider(resource=resource)
   provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(
       endpoint=f"{os.environ['TRACEWAY_URL'].rstrip('/')}/api/otel/v1/traces",
       headers={"Authorization": f"Bearer {os.environ['TRACEWAY_TOKEN']}"},
   )))
   trace.set_tracer_provider(provider)   # import this module before the first model call


   def flush() -> None:
       """For scripts and jobs, before exit (point 5). Never raises."""
       try:
           provider.force_flush(timeout_millis=5000)
       except Exception:
           pass
   ```

   Node: `@opentelemetry/sdk-node` with `@opentelemetry/exporter-trace-otlp-proto` (`new OTLPTraceExporter({ url, headers })`) and `instrumentations: []`, loaded with `--import` before the application; declare `@opentelemetry/api` as a dependency of the application as well, since the helper imports it. Other languages: the per-language notes in SKILL.md Step 4 name the packages; take the SDK and the OTLP/HTTP trace exporter from them and leave the auto-instrumentation out.

   Two things follow from tracing only the model calls. Without a request or job span above it, each model call is the root of its own trace, which is fine for AI Traces: nothing needs a parent. If the application already instruments incoming requests, their active context links the new spans to the corresponding Endpoint.
3. The token must belong to an **OpenTelemetry** (backend) project. With a browser or React Native project's token Traceway stores the spans but creates no AI Trace. A model call made directly from a browser or a mobile app cannot be tracked this way; it has to go through the user's backend first, so report it instead of instrumenting it.
4. If the inventory finds a backend model call and the confirmed plan needs a credential, follow "Invocation Forms" in SKILL.md for that project. An AI feature inside an existing backend belongs in that backend's project.
5. Flush batched spans before a short-lived process exits or a serverless runtime freezes. Flush the tracer provider (`forceFlush()` / `force_flush()` / `ForceFlush(ctx)`), or shut the SDK down when traces are all it exports; a full `sdk.shutdown()` also waits for metric and log exporters, and one of those pointing at nothing makes the script hang for seconds. Export the flush from the module that creates the SDK so the script can call it, and guard it (`try` / `catch`, a timeout): a failed export must not change the job's exit code. In Node, keep the configured span processor if you need its public `forceFlush()` method. The global API tracer provider does not expose that method.

## Step B: Inventory the LLM usage

Find every model call in the requested area or path, following shared wrappers as needed. Search for all three kinds of evidence, because a codebase usually has a wrapper that hides the SDK from a dependency search.

**Dependencies** (`package.json`, `requirements.txt` / `pyproject.toml`, `go.mod`, `pom.xml` / `build.gradle`, `*.csproj`, `composer.json`, `Gemfile`):

- Provider SDKs: `openai`, `@anthropic-ai/sdk` / `anthropic`, `@google/genai` / `google-genai` / `google-generativeai`, `@aws-sdk/client-bedrock-runtime` / `boto3` (Bedrock), `mistralai`, `cohere`, `groq`, `ollama`, `together`, `replicate`, `openai-go`, `anthropic-sdk-go`, `openai-php/client`, `ruby-openai`
- Gateways and abstraction layers: `ai` (Vercel AI SDK) and `@ai-sdk/*`, `litellm`, `langchain` / `@langchain/*`, `llama-index` / `llamaindex`, `spring-ai`, `langchain4j`, `Microsoft.Extensions.AI`, `semantic-kernel`, OpenRouter
- Agent frameworks: `langgraph`, `crewai`, `autogen` / `ag2`, `pydantic-ai`, `@openai/agents` / `openai-agents`, `@mastra/*`, `claude-agent-sdk`
- LLM observability already installed: `traceloop-sdk` / `@traceloop/node-server-sdk` (OpenLLMetry), `openlit`, `openinference-*`, `langfuse`, `langsmith`, `helicone`, `opentelemetry-instrumentation-openai*`, Vercel `experimental_telemetry`
- Things that emit GenAI spans without appearing as an LLM dependency: a provider SDK with tracing built in (versions of the `anthropic` Python SDK with native OpenTelemetry support emit their own spans when a tracer provider exists), and auto-instrumentation bundles (`@opentelemetry/auto-instrumentations-node` contains an OpenAI instrumentation, `opentelemetry-bootstrap -a install` adds one for each LLM package it finds)

**Call sites**: `chat.completions.create`, `responses.create`, `messages.create`, `messages.stream`, `embeddings.create`, `generateText` / `streamText` / `generateObject` / `embed`, `generate_content` / `generateContent`, `converse` / `invoke_model`, `.invoke(` / `.ainvoke(` / `.stream(` on a chain or model object, `completion(` from litellm.

**Raw HTTP**: requests to `api.openai.com`, `api.anthropic.com`, `openrouter.ai`, `generativelanguage.googleapis.com`, `bedrock-runtime`, `api.mistral.ai`, `localhost:11434` (Ollama), or to a base URL read from a variable such as `OPENAI_BASE_URL`, `ANTHROPIC_BASE_URL`, `LLM_BASE_URL`.

For each call site record what the instrumentation will need:

| Call site | SDK / provider | Operation | Streaming | Tools | Runs inside | Conversation id in scope | User id in scope | Already instrumented |
|---|---|---|---|---|---|---|---|---|
| `src/llm/client.ts:41` (shared by 6 callers) | `openai` / OpenAI | chat | yes, SSE | yes, loop in `agent.ts` | `POST /api/chat` | `chatId` from the request | `req.user.id` | no |
| `jobs/summarize.py:18` | `anthropic` / Anthropic | chat | no | no | nightly cron | none, one-shot | `account_id` on the job | no |

Identify existing GenAI instrumentation from the installed SDK versions, framework hooks, instrumentation configuration and any available test spans. Record anything that already emits one span per model call. Check this with a local fixture in Step D before adding manual spans.

Also work out, since they decide the design:

- **Is there one choke point?** A shared client module or wrapper that most calls pass through is where the instrumentation goes. Six call sites behind one wrapper are one change, not six.
- **Is it conversational?** Chat or streaming-chat routes, stored message history or thread ids, tool definitions passed to the model, or an agent framework. If so, read [ai-agent.md](ai-agent.md) before designing: it covers what the conversation id and user id must be, tool-call payload shapes and sub-agents.
- **Where does the model name come from?** A constant, config, or a per-request choice. It names the span and picks the price.
- **Which feature is each call?** Give each one a stable name ("Support Chat Agent", "Nightly Summary"). That name is `trace.name`, the grouping key of `/ai-traces`.

Show the inventory, proposed changes and destination. Wait for confirmation before configuring an exporter, running model calls for verification or editing the integration. Ask about content capture if the user has not already specified it: **may prompts and completions leave the application?** Traceway uses captured content for its chat view, tool-call counts from completions, and content flagging. Without it everything about tokens, cost, latency and errors still works. Ask once, then wire the answer as configuration (Step D), not as code.

## Step C: Choose how to instrument

| Situation | Approach |
|---|---|
| The calls go through one wrapper or client module | Instrument inside it. Callers pass the feature name and, where they have them, the conversation and user ids. |
| Calls are scattered and use the provider SDK directly | Add one small helper per provider (`tracedChat`, `traced_message`) and route every call site through it. Do not paste span code into each call site: the cost map and the content switch must live in one place. |
| A call is a plain HTTP request to the provider (`fetch`, `requests`, `http.Client`) | Wrap the existing request. Map the provider’s usage fields, messages and tool calls into the attributes and capture shapes below. |
| A gateway or framework makes the calls (Vercel AI SDK, LangChain, LiteLLM, an agent framework) | Use its hooks (callbacks, middleware, `onFinish`) to create the same span, or an instrumentation library plus the checks below. |
| An LLM instrumentation library is already installed | Keep it only if its spans pass "Self-check a span" in Step E. See the next paragraph. |
| The app only calls OpenRouter and wants no code change | OpenRouter's Broadcast feature can send the spans itself: see "OpenRouter zero-code path" in [ai-agent.md](ai-agent.md). |

**Instrumentation libraries are a starting point, never the whole job.** Judge one by the spans it emits, not by its name:

- No attribute starting with `gen_ai.` (OpenInference, Langfuse and LangSmith exporters use `llm.*`, `input.value` or vendor prefixes): Traceway stores a plain span and no AI Trace. Replace the library, or map its attributes in a span processor.
- Content on the spec's `gen_ai.input.messages` / `gen_ai.output.messages`, on indexed keys such as `gen_ai.prompt.0.content`, or on `ai.prompt.messages`: tokens look right, the chat view is empty and tool calls count 0.
- Check which cost and identity attributes the library supplies. Add any missing prices, conversation id, user id and feature name at the call site or through its instrumentation hooks.
- Many emit `gen_ai.*` on tool and agent spans, which Traceway counts as extra model calls.

Use a library hook or exporter-side transformation for missing attributes. If another backend uses the original attributes, apply Traceway-specific mappings only on its export branch. A manual span is an option when the application owns the call site.

## Where Traceway differs from the spec

Follow the spec for everything not listed here.

| Topic | The spec says | Send to Traceway | If you only follow the spec |
|---|---|---|---|
| Content | `gen_ai.input.messages`, `gen_ai.output.messages`, `gen_ai.system_instructions` | `gen_ai.prompt` and `gen_ai.completion`, as JSON strings in the provider's own shape (below). Do not also send the spec attributes. | No chat view, 0 tool calls, no content flagging. Sending the content under both sets of keys duplicates it on the exported span. |
| Cost | Nothing | `gen_ai.usage.input_cost`, `gen_ai.usage.output_cost` (USD, doubles). `gen_ai.usage.total_cost` is their sum unless you set it. | Every cost figure is 0.00. Traceway has no price list. |
| Grouping | Span name `{operation} {model}` | Keep that span name, and add `trace.name` with the feature or agent name | `/ai-traces` groups by span name, so every feature using one model collapses into a row called `chat gpt-4o` |
| End user | Nothing in the GenAI spec (`user.id` is a general attribute) | `user.id` on every model-call span | The Users tab is empty |
| Tool execution | An `execute_tool {tool}` span with `gen_ai.operation.name` and `gen_ai.tool.*` | The same span name and kind (`INTERNAL`), with no attribute starting with `gen_ai.`. Use `tool.name`, `tool.call.id`. | Each tool run becomes its own AI Trace row with 0 tokens, an extra turn in its conversation, and a tool call counted a second time |
| Agent and workflow | `invoke_agent {agent}` / `create_agent` / `invoke_workflow` spans with `gen_ai.*` | The same span name, with no attribute starting with `gen_ai.`. Use `agent.name`. The agent's name goes on its model calls as `trace.name`. | Each agent invocation is a 0-token AI Trace row and an extra turn |

The rule behind the last two rows: **on a backend project, a span with a `gen_ai.` attribute becomes an AI Trace after the endpoint, task and exception classification rules below have been applied.** Only spans that are one provider request may carry one.

Two more things that fail without an error:

- **Numbers must be numbers.** Token counts are integer attributes and costs are doubles. A count sent as the string `"152"` is stored as 0.
- **The span kind is `CLIENT`**, as the spec says. Not `INTERNAL`: an `INTERNAL` span with `exception.*` attributes is never promoted, so a failed call would lose its AI Trace. And not the request's `SERVER` span or a job's `CONSUMER` span: those are an Endpoint and a Task, and `gen_ai.*` attributes on them are ignored for AI Traces. The model call is always a span of its own.

## The span, attribute by attribute

One span per provider request, started before the request and ended when the response (or the stream) is complete.

| Attribute | Value | Notes |
|---|---|---|
| Span name | `{gen_ai.operation.name} {gen_ai.request.model}`, e.g. `chat gpt-4o` | |
| Span kind | `CLIENT` | |
| `gen_ai.operation.name` | `chat`, `embeddings`, `text_completion`, `generate_content` | Required by the spec |
| `gen_ai.provider.name` | `openai`, `anthropic`, `aws.bedrock`, `azure.ai.openai`, `gcp.gemini`, `gcp.vertex_ai`, `mistral`, `cohere`, `groq`, `deepseek`, `x_ai`, `perplexity` | Required by the spec. The older `gen_ai.system` is read too. For a gateway use the provider that serves the model when you know it. |
| `gen_ai.request.model` | The model you asked for | |
| `gen_ai.response.model` | The model that answered | From the response |
| `gen_ai.response.id` | The provider's response id | Kept in the trace's attribute list, useful for support tickets |
| `gen_ai.response.finish_reasons` | String array, e.g. `["stop"]`, `["tool_calls"]` | |
| `gen_ai.usage.input_tokens` | Integer, including cached tokens | Anthropic reports cache reads and cache writes separately from `input_tokens`: add all three |
| `gen_ai.usage.output_tokens` | Integer | |
| `gen_ai.usage.cache_read.input_tokens` | Integer, when the provider reports cache reads | `gen_ai.usage.input_tokens.cached` is read too |
| `gen_ai.usage.reasoning.output_tokens` | Integer, when the provider reports reasoning tokens | `gen_ai.usage.output_tokens.reasoning` is read too |
| `gen_ai.usage.input_cost` / `gen_ai.usage.output_cost` | Doubles, USD | From your price map |
| `trace.name` | Stable feature or agent name | No ids, no model names, no timestamps |
| `gen_ai.conversation.id` | The app's own chat or thread id | Only when the call belongs to a conversation. Never invent one per request: without it Traceway falls back to `session.id` on the span or resource, then to the trace id. |
| `user.id` | Stable id of the customer the call is made for | An account or tenant id, not a session id. Stored and searchable, so use an internal id or a hash when emails must stay out. Leave it off when the code only has a placeholder such as `anonymous`, and off calls made for nobody in particular (a shared index build, a nightly job). When a route only knows the staff member who triggered the call, use that and say so in the report. |
| `gen_ai.prompt` | JSON string `{"messages":[...]}` in the provider's request shape | Only with content capture on |
| `gen_ai.completion` | JSON string of the provider's response: `{"choices":[...]}` for OpenAI-style APIs, the whole message object (`{"content":[...], ...}`) for Anthropic | Only with content capture on. For APIs with a different response schema, map their messages and tool calls into one of these shapes for telemetry. For example, an OpenAI Responses payload has an `output` list. Map assistant text and `function_call` items to `choices[0].message.content` and `tool_calls`, retaining each `call_id` for its tool result. Preserve the original response returned to the application. |
| `error.type` | The exception class or the provider's error code | On failure, together with `recordException` and span status `ERROR` |
| `gen_ai.request.temperature`, `gen_ai.request.max_tokens`, `server.address`, ... | As the spec defines them | Optional. Shown in the trace's attribute list. |

Set the request-side attributes (operation, provider, request model, `trace.name`, conversation and user ids, prompt) when the span starts, so a failed call still says which feature, user and conversation it belonged to. Set the response-side ones when the response arrives.

**Embeddings** use the same span with `gen_ai.operation.name = "embeddings"`, the span name `embeddings {model}`, `gen_ai.usage.input_tokens` from the response's usage and `gen_ai.usage.input_cost`. There are no output tokens, no finish reason and no content.

**Anthropic system prompt**: the API takes `system` outside `messages`. Put it first in the captured prompt as `{"role":"system","content":"..."}` so the chat view shows it.

## Step D: Implement

After the scoped plan is confirmed, establish the export path from Step A. Take a baseline with a synthetic request and an in-memory exporter before adding a manual span. If the SDK or a framework already produces a model-call span, enrich it using supported hooks or disable that overlapping instrumentation when implementing the manual span. Preserve other consumers of an existing exporter. Re-run the fixture to check that the Traceway export path produces exactly one model-call span per call.

Content capture is read from the spec's own variable, `OTEL_INSTRUMENTATION_GENAI_CAPTURE_CONTENT`. Capture when its value is `SPAN_ONLY` or `SPAN_AND_EVENT` (case-insensitive); the default, `NO_CONTENT`, captures nothing. Add it to the env files and deploy config according to the user's answer in Step B.

TypeScript, OpenAI-style chat:

```typescript
import { context, SpanKind, SpanStatusCode, trace, type Span } from "@opentelemetry/api";
import type OpenAI from "openai";

const tracer = trace.getTracer("llm");

// USD per token. Example numbers: take the real ones from the provider's pricing page.
// A model missing here reports cost 0.00, so log it once when that happens.
const PRICES: Record<string, { input: number; cachedInput?: number; output: number }> = {
  "gpt-4o": { input: 2.5e-6, cachedInput: 1.25e-6, output: 10e-6 },
};

const captureContent = /^(span_only|span_and_event)$/i.test(
  process.env.OTEL_INSTRUMENTATION_GENAI_CAPTURE_CONTENT ?? "",
);

export type LlmCall = { feature: string; conversationId?: string; userId?: string };

function startChatSpan(call: LlmCall, params: { model: string; messages: unknown }): Span {
  const span = tracer.startSpan(`chat ${params.model}`, { kind: SpanKind.CLIENT });
  span.setAttributes({
    "gen_ai.operation.name": "chat",
    "gen_ai.provider.name": "openai",
    "gen_ai.request.model": params.model,
    "trace.name": call.feature,
  });
  if (call.conversationId) span.setAttribute("gen_ai.conversation.id", call.conversationId);
  if (call.userId) span.setAttribute("user.id", call.userId);
  if (captureContent) span.setAttribute("gen_ai.prompt", JSON.stringify({ messages: params.messages }));
  return span;
}

function recordCompletion(span: Span, requestModel: string, completion: OpenAI.ChatCompletion) {
  const usage = completion.usage;
  const input = usage?.prompt_tokens ?? 0;
  const output = usage?.completion_tokens ?? 0;
  const cached = usage?.prompt_tokens_details?.cached_tokens ?? 0;
  const reasoning = usage?.completion_tokens_details?.reasoning_tokens ?? 0;
  const price = PRICES[completion.model] ?? PRICES[requestModel];
  span.setAttributes({
    "gen_ai.response.model": completion.model,
    "gen_ai.response.id": completion.id,
    "gen_ai.response.finish_reasons": completion.choices.map((c) => c.finish_reason),
    "gen_ai.usage.input_tokens": input,
    "gen_ai.usage.output_tokens": output,
    "gen_ai.usage.cache_read.input_tokens": cached,
    "gen_ai.usage.reasoning.output_tokens": reasoning,
  });
  if (price) {
    span.setAttributes({
      "gen_ai.usage.input_cost": (input - cached) * price.input + cached * (price.cachedInput ?? price.input),
      "gen_ai.usage.output_cost": output * price.output,
    });
  }
  if (captureContent) span.setAttribute("gen_ai.completion", JSON.stringify({ choices: completion.choices }));
}

function recordFailure(span: Span, error: unknown) {
  const err = error instanceof Error ? error : new Error(String(error));
  // The provider's error code when there is one ("model_not_found", "rate_limit_exceeded"),
  // else the class. The OpenAI SDK leaves `name` at "Error" on all of its error classes.
  const code = (error as { code?: unknown } | null)?.code;
  span.recordException(err);
  span.setAttribute("error.type", typeof code === "string" ? code : err.constructor.name);
  span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
}

// Runs the provider request with the model span active, so the HTTP request span of the
// auto-instrumentation becomes its child. Only the request goes in here: tools run after the span ended.
function inSpan<T>(span: Span, request: () => Promise<T>): Promise<T> {
  return context.with(trace.setSpan(context.active(), span), request);
}

export async function tracedChat(
  openai: OpenAI,
  call: LlmCall,
  params: OpenAI.ChatCompletionCreateParamsNonStreaming,
) {
  const span = startChatSpan(call, params);
  try {
    const completion = await inSpan(span, () => openai.chat.completions.create(params));
    recordCompletion(span, params.model, completion);
    return completion;
  } catch (error) {
    recordFailure(span, error);
    throw error;
  } finally {
    span.end();
  }
}
```

`tracer.startSpan` picks up the active context, so inside a request or a job the span is a child of that Endpoint or Task and the two link automatically. Outside any active span it is a root span, which is fine: an AI Trace needs no parent. The application must declare `@opentelemetry/api` in its own dependencies; do not rely on the copy the SDK packages pull in.

Python, Anthropic:

```python
import json
import os
from contextlib import contextmanager

import anthropic
from opentelemetry import trace
from opentelemetry.trace import SpanKind

tracer = trace.get_tracer("llm")

# Reuse the application client. For SDK versions with native tracing, set
# ANTHROPIC_OPEN_TELEMETRY=false at startup if manual spans replace it.
client = anthropic.Anthropic()

# USD per token. Example numbers: take the real ones from the provider's pricing page.
# A model missing here reports no cost, so log it once when that happens.
PRICES = {
    "claude-sonnet-5-5": {
        "input": 2e-6, "cache_read": 0.1e-6, "write_5m": 2.5e-6, "write_1h": 4e-6, "output": 10e-6,
    },
}

CAPTURE_CONTENT = os.getenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_CONTENT", "").upper() in (
    "SPAN_ONLY",
    "SPAN_AND_EVENT",
)


def _start(span, feature, conversation_id, user_id, params):
    span.set_attributes({
        "gen_ai.operation.name": "chat",
        "gen_ai.provider.name": "anthropic",
        "gen_ai.request.model": params["model"],
        "trace.name": feature,
    })
    if conversation_id:
        span.set_attribute("gen_ai.conversation.id", conversation_id)
    if user_id:
        span.set_attribute("user.id", user_id)
    if CAPTURE_CONTENT:
        # The SDK accepts iterables. Materialize them once for both capture and the request.
        params["messages"] = list(params["messages"])
        messages = params["messages"].copy()
        if params.get("system"):
            messages.insert(0, {"role": "system", "content": params["system"]})
        try:
            prompt = json.dumps({"messages": messages}, default=lambda block: block.model_dump(mode="json"))
        except (AttributeError, TypeError, ValueError):
            # Some SDK inputs (such as file handles) have no JSON representation.
            return
        span.set_attribute("gen_ai.prompt", prompt)


def _finish(span, request_model, response):
    usage = response.usage
    cache_read = getattr(usage, "cache_read_input_tokens", None) or 0
    cache_write = getattr(usage, "cache_creation_input_tokens", None) or 0
    span.set_attributes({
        "gen_ai.response.model": response.model,
        "gen_ai.response.id": response.id,
        "gen_ai.response.finish_reasons": [response.stop_reason or ""],
        "gen_ai.usage.input_tokens": usage.input_tokens + cache_read + cache_write,
        "gen_ai.usage.output_tokens": usage.output_tokens,
        "gen_ai.usage.cache_read.input_tokens": cache_read,
    })
    price = PRICES.get(response.model) or PRICES.get(request_model)
    if price:
        cache_detail = getattr(usage, "cache_creation", None)
        write_1h = getattr(cache_detail, "ephemeral_1h_input_tokens", 0) or 0
        span.set_attributes({
            "gen_ai.usage.input_cost": usage.input_tokens * price["input"]
            + cache_read * price["cache_read"]
            + (cache_write - write_1h) * price["write_5m"] + write_1h * price["write_1h"],
            "gen_ai.usage.output_cost": usage.output_tokens * price["output"],
        })
    if CAPTURE_CONTENT:
        span.set_attribute("gen_ai.completion", response.model_dump_json())


@contextmanager
def message_span(feature, conversation_id, user_id, params):
    with tracer.start_as_current_span(f"chat {params['model']}", kind=SpanKind.CLIENT) as span:
        _start(span, feature, conversation_id, user_id, params)
        try:
            yield span
        except Exception as exc:
            span.set_attribute("error.type", type(exc).__qualname__)
            raise


def traced_message(*, feature, conversation_id=None, user_id=None, **params):
    with message_span(feature, conversation_id, user_id, params) as span:
        response = client.messages.create(**params)
        _finish(span, params["model"], response)
        return response
```

The Python price map uses standard synchronous USD rates. Match the deployed model, cache durations, region and service tier when estimating costs. [Anthropic pricing](https://platform.claude.com/docs/en/about-claude/pricing)

Other languages use the same attributes with the language's OTel API: Go `tracer.Start(ctx, "chat "+model, trace.WithSpanKind(trace.SpanKindClient))`, Java `tracer.spanBuilder(...).setSpanKind(SpanKind.CLIENT)`, .NET `ActivitySource.StartActivity(name, ActivityKind.Client)`, PHP `->setSpanKind(SpanKind::KIND_CLIENT)`.

### Streaming

For streamed Chat Completions and Anthropic Messages, the span covers the whole stream: start it before the request, end it when the last chunk has arrived, the stream failed, or the consumer stopped reading. Ending it when the call returns the stream object records a few milliseconds and no tokens.

- **Read final usage when the stream completes.** OpenAI-style streams report it in the final chunk and only when the request sets `stream_options: { include_usage: true }`; add that to the request. Anthropic's stream carries it in the final message.
- **Rebuild the response for `gen_ai.completion`.** Prefer the SDK's own accumulator (`stream.finalChatCompletion()` / `get_final_completion()` for OpenAI, `stream.finalMessage()` / `get_final_message()` for Anthropic, `onFinish` in the Vercel AI SDK). When the code iterates raw chunks, accumulate the text and the tool-call deltas (by `index`) yourself and write the result in the same shape a non-streaming response has.
- **Use `try` / `finally`** around the iteration, in a generator when the stream is passed on to a caller, so the span ends on every path including an aborted client.

```typescript
export async function* tracedChatStream(
  openai: OpenAI,
  call: LlmCall,
  params: OpenAI.ChatCompletionCreateParamsStreaming,
) {
  const span = startChatSpan(call, params);
  try {
    const stream = await inSpan(span, async () =>
      openai.chat.completions.stream({
        ...params, stream_options: { ...params.stream_options, include_usage: true },
      }),
    );
    for await (const chunk of stream) yield chunk;
    recordCompletion(span, params.model, await stream.finalChatCompletion());
  } catch (error) {
    recordFailure(span, error);
    throw error;
  } finally {
    span.end();
  }
}
```

For an Anthropic event loop, wrap the existing stream context and record the accumulated message on `message_stop`. Exiting early closes the stream immediately. Calling `get_final_message()` during cleanup would consume the remaining events even after cancellation. [Anthropic Python streaming implementation](https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/lib/streaming/_messages.py)

```python
with message_span(feature, conversation_id, user_id, params) as span:
    with client.messages.stream(**params) as stream:
        for event in stream:
            if event.type == "message_stop":
                _finish(span, params["model"], event.message)
            # Keep the application's event handling and early exits here.
```

### Tool calls, agents and retries

- **Each iteration of a tool-calling loop is its own model call and its own span.** Run the tools after the model span has ended, each in an `execute_tool <name>` span of kind `INTERNAL` with `tool.name`, `tool.call.id` and no `gen_ai.*` attribute. Tool arguments and results are content: add them (`tool.args`, `tool.result`) only when content capture is on. The tool spans then sit next to the model calls in the request's or job's waterfall. Tool names and counts on the AI Trace come from the completion payload, so they need content capture.
- **A wrapper span around an agent run is optional.** Inside a request or job span the model calls and tool spans already share a parent, so add none. Add an `invoke_agent <name>` span (plain, without `gen_ai.*`, with `agent.name`) when nothing else is above the loop, so the calls and tools of one run share a trace, or when one request runs several agents and the waterfall should show which calls belong to which. Either way every model call carries its agent's name as `trace.name`.
- **Retries**: when your code retries, each attempt is a span. When the SDK retries internally you see one span covering all attempts, which is what the spec asks for.
- **Never put `gen_ai.*` on the HTTP route span or the job span**, not even the conversation id. Pass the ids down to the model-call span instead: as arguments, or through request-scoped context (`AsyncLocalStorage`, `contextvars`, a `context.Context` value).

## Step E: Verify

### Self-check a span

Run a synthetic response through each instrumented helper using a test tracer provider and an in-memory exporter. Include a streaming response and a simulated provider failure. Use the installed SDK's span processor API. A local fixture verifies the span contract. When verifying live delivery, use the application's configured test environment and synthetic data. Production calls, billable provider requests and deliberate issues require a destination and test activity agreed with the user.

1. **Exactly one span with `gen_ai.*` attributes per provider request.** Two means an SDK or instrumentation library traces the call as well (Step D, baseline).
2. Kind is `CLIENT`, name is `<operation> <model>`.
3. `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model` and `trace.name` are set.
4. `gen_ai.usage.input_tokens` and `gen_ai.usage.output_tokens` are integer attributes matching the provider’s usage. (Embeddings: input tokens only.)
5. `gen_ai.usage.input_cost` and `gen_ai.usage.output_cost` are numeric USD amounts matching the applicable rates and usage. Free calls and zero-token categories can legitimately cost zero. For embeddings check input cost only.
6. With content capture on: `gen_ai.prompt` parses as JSON with a `messages` array, and `gen_ai.completion` parses as JSON with `choices` (OpenAI-style) or `content` (Anthropic).
7. No `gen_ai.input.messages` / `gen_ai.output.messages`, and no `gen_ai.*` key on any span that is not a model call (tools, agent wrappers, the request span).
8. For a completed stream, check its full duration and reported usage. For an early exit, check that the span ends when the consumer stops and that the wrapper does not read additional events.
9. For the simulated failure, confirm the span has status `ERROR`, `error.type`, an `exception` event, and still its `trace.name`, conversation and user.

To check the ingest URL and token in the agreed environment, post an empty payload with the application's own variables:

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$TRACEWAY_URL/api/otel/v1/traces" \
  -H "Authorization: Bearer $TRACEWAY_TOKEN" -H 'Content-Type: application/json' -d '{}'
```

`200` means the URL and token are right. Verify delivery separately with a test span from the configured application and a dashboard or authenticated CLI read.

**What an agent with only the ingest token cannot check.** The token writes and never reads, so whether the spans became AI Traces can only be seen in the dashboard, or through the `traceway` CLI once the user is logged in. If read access is unavailable, report which local checks passed and give the user the feature name and synthetic ids to look for on the dashboard.

### In Traceway

Set the time picker to the last 15 minutes. The lists are dashboard pages; with the `traceway` CLI logged in, `traceway ai-traces show <id> --recorded-at <timestamp> --output json` retrieves the call and captured conversation. Use the AI Trace UUID and the RFC 3339 timestamp from the detail URL’s `t` parameter.

| Check | Expected | When it is wrong |
|---|---|---|
| `/ai-traces`, Traces tab | One row per feature name, with call counts matching the model spans exported | Nothing at all: no span carries a `gen_ai.*` attribute, the attributes sit on the route or job span, or the token belongs to a browser project. Rows named `chat <model>`: `trace.name` is missing. Rows named `execute_tool ...` or `invoke_agent ...`: those spans carry `gen_ai.*`. |
| Open a feature, then one call | Model, provider, token counts, cost, finish reason | Tokens 0: sent as strings, or a stream ended before usage arrived. Cost 0.00: no cost attributes or a model missing from the price map. Cached or reasoning tokens 0: only the spec names were sent. |
| The call's Conversation card | A chat view with the messages and any tool calls | No card: content capture is off, or the content is on the spec attribute names. Raw JSON instead of a chat: the prompt is not `{"messages":[...]}` or the completion is not the provider's response shape. |
| The call's Spans card, and the Endpoint or Task that made the call | The provider HTTP request under the call, when HTTP client instrumentation is on and the request ran with the model span active; the call and its tool spans in the parent's waterfall | Call missing from the parent's waterfall: the span was started outside the request's context. No Endpoint or Task at all: nothing traces the request, which a scoped AI run does not add. |
| `/issues` if the test failure was exported | One issue, and the failed call listed as an error row on `/ai-traces` | Issue but no AI Trace: the span kind is `INTERNAL`. |
| Conversational products | The checks in "Verify" of [ai-agent.md](ai-agent.md) | |

## Report

Finish a scoped run with: the inventory (every call site and whether it is now traced), the files changed, the price map and which models it covers, whether content capture is on and the variable that controls it, what you verified and how, what you could not verify, and any test data exported to the project (feature rows, conversation ids, issues). List what you saw outside the scope without changing it, for example "background jobs are not instrumented; run `/traceway-setup instrument background jobs`". Mention the three alert rules AI data enables (single-call cost, 24h conversation cost, flagged content), described at the end of [ai-agent.md](ai-agent.md).
