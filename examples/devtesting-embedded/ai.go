package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// An AI chat agent backed by OpenRouter, instrumented with gen_ai.* span
// attributes so every LLM call lands in Traceway's AI Traces / Conversations
// analytics: conversation grouping (gen_ai.conversation.id), per-user stats
// (user.id), tool calls parsed from the completion payload, and sub-agents as
// separate trace names inside the same conversation.

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

var aiHTTPClient = &http.Client{Timeout: 120 * time.Second}

// --- OpenRouter wire types (OpenAI chat-completions compatible) ---

type orToolCall struct {
	Id       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type orMessage struct {
	Role       string       `json:"role"`
	Content    *string      `json:"content,omitempty"`
	ToolCalls  []orToolCall `json:"tool_calls,omitempty"`
	ToolCallId string       `json:"tool_call_id,omitempty"`
}

func textMsg(role, content string) orMessage {
	return orMessage{Role: role, Content: &content}
}

type orTool struct {
	Type     string    `json:"type"`
	Function orToolDef `json:"function"`
}

type orToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type orRequest struct {
	Model    string      `json:"model"`
	Messages []orMessage `json:"messages"`
	Tools    []orTool    `json:"tools,omitempty"`
	Usage    struct {
		Include bool `json:"include"`
	} `json:"usage"`
}

type orChoice struct {
	FinishReason string    `json:"finish_reason"`
	Message      orMessage `json:"message"`
}

type orResponse struct {
	Id      string     `json:"id"`
	Model   string     `json:"model"`
	Choices []orChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int64   `json:"prompt_tokens"`
		CompletionTokens int64   `json:"completion_tokens"`
		TotalTokens      int64   `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func mustSchema(v string) json.RawMessage { return json.RawMessage(v) }

// --- Agents ---

type agentDef struct {
	// TraceName becomes trace.name on the gen_ai spans, so each agent shows up
	// as its own entry on the AI Traces page while sharing the conversation.
	TraceName    string
	SystemPrompt string
	Tools        []orTool
	// MaxTurns bounds the tool-call loop per user message.
	MaxTurns int
}

var mainAgent = agentDef{
	TraceName: "Support Chat Agent",
	SystemPrompt: "You are a helpful support assistant for the Acme web shop. " +
		"Use the available tools whenever they can answer the question: weather, order lookups, server time. " +
		"Delegate research questions (documentation, product knowledge) to the research agent and " +
		"math/calculation questions to the math agent instead of answering from memory. " +
		"Keep answers short and conversational.",
	Tools: []orTool{
		{Type: "function", Function: orToolDef{
			Name:        "get_weather",
			Description: "Get the current weather for a city.",
			Parameters:  mustSchema(`{"type":"object","properties":{"city":{"type":"string","description":"City name"}},"required":["city"]}`),
		}},
		{Type: "function", Function: orToolDef{
			Name:        "lookup_order",
			Description: "Look up the status of a customer order by its order id.",
			Parameters:  mustSchema(`{"type":"object","properties":{"order_id":{"type":"string","description":"The order id, e.g. A-1042"}},"required":["order_id"]}`),
		}},
		{Type: "function", Function: orToolDef{
			Name:        "get_server_time",
			Description: "Get the current server date and time.",
			Parameters:  mustSchema(`{"type":"object","properties":{}}`),
		}},
		{Type: "function", Function: orToolDef{
			Name:        "ask_research_agent",
			Description: "Delegate a research or documentation question to the research sub-agent. It has access to the knowledge base.",
			Parameters:  mustSchema(`{"type":"object","properties":{"question":{"type":"string"}},"required":["question"]}`),
		}},
		{Type: "function", Function: orToolDef{
			Name:        "ask_math_agent",
			Description: "Delegate a math or calculation problem to the math sub-agent. It has a calculator.",
			Parameters:  mustSchema(`{"type":"object","properties":{"problem":{"type":"string"}},"required":["problem"]}`),
		}},
	},
	MaxTurns: 6,
}

var researchAgent = agentDef{
	TraceName: "Research Sub-Agent",
	SystemPrompt: "You are a research sub-agent. Answer the question using the search_knowledge_base tool, " +
		"then reply with a concise summary of what you found. Always search before answering.",
	Tools: []orTool{
		{Type: "function", Function: orToolDef{
			Name:        "search_knowledge_base",
			Description: "Search the internal knowledge base for documentation snippets.",
			Parameters:  mustSchema(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		}},
	},
	MaxTurns: 4,
}

var mathAgent = agentDef{
	TraceName: "Math Sub-Agent",
	SystemPrompt: "You are a math sub-agent. Use the calculate tool for any arithmetic instead of computing yourself, " +
		"then reply with just the result and one short sentence.",
	Tools: []orTool{
		{Type: "function", Function: orToolDef{
			Name:        "calculate",
			Description: "Evaluate an arithmetic expression with + - * / and parentheses, e.g. \"12*(3+4)\".",
			Parameters:  mustSchema(`{"type":"object","properties":{"expression":{"type":"string"}},"required":["expression"]}`),
		}},
	},
	MaxTurns: 4,
}

// --- Chat session context threaded through every LLM/tool call ---

type chatContext struct {
	svc            *otelService
	model          string
	conversationId string
	userId         string
	events         []toolEvent
}

type toolEvent struct {
	Agent  string `json:"agent"`
	Tool   string `json:"tool"`
	Args   string `json:"args"`
	Result string `json:"result"`
}

// callOpenRouter makes one chat-completions call and emits the gen_ai.* span
// Traceway promotes to an ai_traces row.
func (cc *chatContext) callOpenRouter(ctx context.Context, agent agentDef, messages []orMessage) (*orResponse, error) {
	reqBody := orRequest{Model: cc.model, Messages: messages, Tools: agent.Tools}
	reqBody.Usage.Include = true

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	spanCtx, span := cc.svc.tr.Start(ctx, "chat "+cc.model, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	promptJSON, _ := json.Marshal(map[string]any{"messages": messages})
	span.SetAttributes(
		attribute.String("trace.name", agent.TraceName),
		attribute.String("gen_ai.conversation.id", cc.conversationId),
		attribute.String("user.id", cc.userId),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.system", "openrouter"),
		attribute.String("gen_ai.request.model", cc.model),
		attribute.String("gen_ai.prompt", string(promptJSON)),
	)

	httpReq, err := http.NewRequestWithContext(spanCtx, http.MethodPost, openRouterURL, bytes.NewReader(payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("OPENROUTER_API_KEY"))
	httpReq.Header.Set("X-Title", "traceway devtesting-embedded")

	httpResp, err := aiHTTPClient.Do(httpReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var resp orResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("openrouter returned non-JSON (%d): %.300s", httpResp.StatusCode, string(body))
	}
	if resp.Error != nil {
		span.SetStatus(codes.Error, resp.Error.Message)
		return nil, fmt.Errorf("openrouter error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		span.SetStatus(codes.Error, "no choices in response")
		return nil, fmt.Errorf("openrouter returned no choices (%d): %.300s", httpResp.StatusCode, string(body))
	}

	completionJSON, _ := json.Marshal(map[string]any{"choices": resp.Choices})
	span.SetAttributes(
		attribute.String("gen_ai.response.model", resp.Model),
		attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
		attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
		attribute.Int64("gen_ai.usage.total_tokens", resp.Usage.TotalTokens),
		attribute.String("gen_ai.response.finish_reason", resp.Choices[0].FinishReason),
		attribute.String("gen_ai.completion", string(completionJSON)),
	)
	if resp.Usage.Cost > 0 {
		span.SetAttributes(attribute.Float64("gen_ai.usage.total_cost", resp.Usage.Cost))
	}

	return &resp, nil
}

// runAgent drives one agent's tool-call loop and returns its final text reply.
func (cc *chatContext) runAgent(ctx context.Context, agent agentDef, userContent string, history []orMessage) (string, error) {
	messages := []orMessage{textMsg("system", agent.SystemPrompt)}
	messages = append(messages, history...)
	messages = append(messages, textMsg("user", userContent))

	for turn := 0; turn < agent.MaxTurns; turn++ {
		resp, err := cc.callOpenRouter(ctx, agent, messages)
		if err != nil {
			return "", err
		}

		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			if msg.Content != nil {
				return *msg.Content, nil
			}
			return "", nil
		}

		messages = append(messages, msg)
		for _, call := range msg.ToolCalls {
			result := cc.executeTool(ctx, agent, call.Function.Name, call.Function.Arguments)
			messages = append(messages, orMessage{
				Role:       "tool",
				Content:    &result,
				ToolCallId: call.Id,
			})
		}
	}
	return "I hit my tool-call limit before finishing. Please try a simpler question.", nil
}

// executeTool runs a fake tool (or a sub-agent) inside a plain child span.
// Tool spans deliberately carry no gen_ai.* attributes: the tool calls are
// already captured on the LLM rows via the completion payload, and gen_ai
// attributes here would promote each execution to its own ai_traces row.
func (cc *chatContext) executeTool(ctx context.Context, agent agentDef, name, args string) string {
	toolCtx, span := cc.svc.tr.Start(ctx, "tool "+name, trace.WithAttributes(
		attribute.String("tool.name", name),
		attribute.String("tool.agent", agent.TraceName),
		attribute.String("tool.args", truncate(args, 500)),
	))
	defer span.End()

	result := cc.dispatchTool(toolCtx, name, args)
	span.SetAttributes(attribute.String("tool.result", truncate(result, 500)))

	cc.events = append(cc.events, toolEvent{
		Agent:  agent.TraceName,
		Tool:   name,
		Args:   truncate(args, 300),
		Result: truncate(result, 300),
	})
	return result
}

func (cc *chatContext) dispatchTool(ctx context.Context, name, args string) string {
	var params map[string]string
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		params = map[string]string{}
	}

	switch name {
	case "get_weather":
		return fakeWeather(params["city"])
	case "lookup_order":
		return fakeOrderStatus(params["order_id"])
	case "get_server_time":
		return time.Now().Format("Monday, 2 Jan 2006 15:04:05 MST")
	case "search_knowledge_base":
		return fakeKnowledgeBase(params["query"])
	case "calculate":
		value, err := evalArithmetic(params["expression"])
		if err != nil {
			return "error: " + err.Error()
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case "ask_research_agent":
		reply, err := cc.runAgent(ctx, researchAgent, params["question"], nil)
		if err != nil {
			return "research agent failed: " + err.Error()
		}
		return reply
	case "ask_math_agent":
		reply, err := cc.runAgent(ctx, mathAgent, params["problem"], nil)
		if err != nil {
			return "math agent failed: " + err.Error()
		}
		return reply
	default:
		return "unknown tool: " + name
	}
}

// --- Fake tool implementations (deterministic per input, no external calls) ---

func pick(seed string, options []string) string {
	h := fnv.New32a()
	h.Write([]byte(seed))
	return options[int(h.Sum32())%len(options)]
}

func fakeWeather(city string) string {
	if city == "" {
		return "error: city is required"
	}
	condition := pick(city, []string{"sunny", "partly cloudy", "overcast", "light rain", "thunderstorms", "windy"})
	h := fnv.New32a()
	h.Write([]byte(city + "-temp"))
	temp := 8 + int(h.Sum32())%22
	return fmt.Sprintf("Weather in %s: %s, %d°C, humidity %d%%.", city, condition, temp, 40+int(h.Sum32())%45)
}

func fakeOrderStatus(orderId string) string {
	if orderId == "" {
		return "error: order_id is required"
	}
	status := pick(orderId, []string{
		"processing in our warehouse",
		"shipped and in transit with DHL",
		"out for delivery today",
		"delivered and signed for",
		"delayed at customs, new ETA in 2 days",
	})
	h := fnv.New32a()
	h.Write([]byte(orderId + "-eta"))
	eta := time.Now().AddDate(0, 0, 1+int(h.Sum32())%5).Format("Mon, 2 Jan")
	return fmt.Sprintf("Order %s is %s. Estimated delivery: %s.", orderId, status, eta)
}

func fakeKnowledgeBase(query string) string {
	q := strings.ToLower(query)
	snippets := []string{}
	if strings.Contains(q, "traceway") || strings.Contains(q, "observab") || strings.Contains(q, "trace") {
		snippets = append(snippets, "[doc:overview] Traceway is an error tracking and monitoring platform: endpoints, exceptions, logs, metrics, AI traces, and session replay in one self-hostable binary.")
	}
	if strings.Contains(q, "ai") || strings.Contains(q, "conversation") || strings.Contains(q, "llm") {
		snippets = append(snippets, "[doc:ai-tracing] AI calls with gen_ai.* attributes are grouped into conversations via gen_ai.conversation.id; tool calls are parsed from the completion payload and per-user analytics key on user.id.")
	}
	if strings.Contains(q, "return") || strings.Contains(q, "refund") || strings.Contains(q, "policy") {
		snippets = append(snippets, "[doc:returns] Acme shop policy: returns accepted within 30 days of delivery, refunds are processed to the original payment method within 5 business days.")
	}
	if strings.Contains(q, "shipping") || strings.Contains(q, "delivery") {
		snippets = append(snippets, "[doc:shipping] Standard shipping takes 2-5 business days; express is next-day when ordered before 15:00.")
	}
	if len(snippets) == 0 {
		snippets = append(snippets, "[doc:misc] No exact match found. Closest entry: the Acme handbook says to escalate unknown questions to a human agent.")
	}
	return strings.Join(snippets, "\n")
}

// evalArithmetic is a tiny recursive-descent evaluator for + - * / and
// parentheses, so the calculate tool actually computes.
func evalArithmetic(input string) (float64, error) {
	p := &exprParser{src: strings.ReplaceAll(input, " ", "")}
	if p.src == "" {
		return 0, fmt.Errorf("empty expression")
	}
	value, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.src) {
		return 0, fmt.Errorf("unexpected character %q at position %d", p.src[p.pos], p.pos)
	}
	return value, nil
}

type exprParser struct {
	src string
	pos int
}

func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
		op := p.src[p.pos]
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.src) && (p.src[p.pos] == '*' || p.src[p.pos] == '/') {
		op := p.src[p.pos]
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *exprParser) parseFactor() (float64, error) {
	if p.pos < len(p.src) && p.src[p.pos] == '-' {
		p.pos++
		value, err := p.parseFactor()
		return -value, err
	}
	if p.pos < len(p.src) && p.src[p.pos] == '(' {
		p.pos++
		value, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return value, nil
	}
	start := p.pos
	for p.pos < len(p.src) && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9' || p.src[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected a number at position %d", start)
	}
	return strconv.ParseFloat(p.src[start:p.pos], 64)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// --- HTTP handler ---

type chatRequest struct {
	SessionId string `json:"sessionId"`
	UserId    string `json:"userId"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func registerAIChatRoutes(router *gin.Engine, svc *otelService) {
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "anthropic/claude-sonnet-5"
	}

	router.POST("/api/ai/chat", func(c *gin.Context) {
		if os.Getenv("OPENROUTER_API_KEY") == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OPENROUTER_API_KEY is not set; add it to examples/devtesting-embedded/.env"})
			return
		}

		var req chatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.SessionId == "" || len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId and messages are required"})
			return
		}
		if req.UserId == "" {
			req.UserId = "anonymous"
		}
		last := req.Messages[len(req.Messages)-1]
		if last.Role != "user" || strings.TrimSpace(last.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the last message must be a non-empty user message"})
			return
		}

		var history []orMessage
		for _, m := range req.Messages[:len(req.Messages)-1] {
			if m.Role == "user" || m.Role == "assistant" {
				history = append(history, textMsg(m.Role, m.Content))
			}
		}

		cc := &chatContext{
			svc:            svc,
			model:          model,
			conversationId: req.SessionId,
			userId:         req.UserId,
		}

		reply, err := cc.runAgent(c.Request.Context(), mainAgent, last.Content, history)
		if err != nil {
			span := trace.SpanFromContext(c.Request.Context())
			span.RecordError(err)
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"reply":  reply,
			"events": cc.events,
			"model":  model,
		})
	})
}
