package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type supportChatRequest struct {
	ConversationId string `json:"conversation_id"`
	UserId         string `json:"user_id"`
	Message        string `json:"message"`
}

var (
	conversationsMu sync.Mutex
	conversations   = map[string][]chatMessage{}
)

const supportModel = "gpt-4o-mini"

const supportSystemPrompt = "You are the Shoply support assistant. Answer questions about orders, returns and products. Use the lookup_order tool when the customer asks about a specific order."

func supportChat(c *gin.Context) {
	ctx := c.Request.Context()
	var req supportChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConversationId == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id and message are required"})
		return
	}

	conversationsMu.Lock()
	history := conversations[req.ConversationId]
	if len(history) == 0 {
		history = []chatMessage{{Role: "system", Content: supportSystemPrompt}}
	}
	history = append(history, chatMessage{Role: "user", Content: req.Message})
	conversationsMu.Unlock()

	slog.InfoContext(ctx, "support chat message received",
		"conversation_id", req.ConversationId, "user_id", req.UserId)

	lower := strings.ToLower(req.Message)
	var reply string

	if strings.Contains(lower, "order") && strings.Contains(strings.ToUpper(req.Message), "ORD-") {
		orderId := extractOrderId(req.Message)

		toolCallsCompletion := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_order","arguments":"{\"order_id\":\"%s\"}"}}]},"finish_reason":"tool_calls"}]}`, orderId)
		emitModelSpan(c, req, history, toolCallsCompletion, "tool_calls")

		toolResult := runLookupOrderTool(c, orderId)

		history = append(history,
			chatMessage{Role: "assistant", Content: fmt.Sprintf("[tool_call lookup_order(%s)]", orderId)},
			chatMessage{Role: "tool", Content: toolResult},
		)
		reply = fmt.Sprintf("I checked order %s for you — it shipped yesterday via DHL and is expected to arrive within 2 business days. Tracking: DH%08d.", orderId, rand.IntN(100000000))
		emitModelSpan(c, req, history, assistantCompletion(reply), "stop")
	} else {
		switch {
		case strings.Contains(lower, "return") || strings.Contains(lower, "refund"):
			reply = "You can return any item within 30 days of delivery for a full refund. Head to Orders → Return item, print the prepaid label, and drop the parcel at any pickup point. Refunds land within 5 business days of us receiving it."
		case strings.Contains(lower, "candle") || strings.Contains(lower, "scent"):
			reply = "The Scented Candle is a cedar and sage soy blend with roughly 40 hours of burn time. It's hand-poured, and the jar is reusable. It's one of our most re-ordered Home items."
		case strings.Contains(lower, "ship"):
			reply = "Standard shipping is free over $50 and takes 2-4 business days. Express (next business day) is $9.90 at checkout."
		case strings.Contains(lower, "bullshit") || strings.Contains(lower, "terrible") || strings.Contains(lower, "angry"):
			reply = "I'm really sorry about the experience — that's not the bar we hold ourselves to. I've flagged this conversation for a senior support agent, and in the meantime I can start a refund or replacement right away. Which would you prefer?"
		default:
			reply = "Happy to help! I can look up an order by its ORD- number, explain our return policy, or answer product questions. What do you need?"
		}
		emitModelSpan(c, req, history, assistantCompletion(reply), "stop")
	}

	history = append(history, chatMessage{Role: "assistant", Content: reply})
	conversationsMu.Lock()
	conversations[req.ConversationId] = history
	conversationsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"reply": reply, "conversation_id": req.ConversationId})
}

func emitModelSpan(c *gin.Context, req supportChatRequest, history []chatMessage, completion, finishReason string) {
	ctx := c.Request.Context()
	promptJSON, _ := json.Marshal(gin.H{"messages": history})
	promptTokens := int64(len(promptJSON) / 4)
	outputTokens := int64(len(completion) / 4)

	_, span := tracer.Start(ctx, "chat "+supportModel, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String("trace.name", "Support Chat Agent"),
		attribute.String("gen_ai.system", "openai"),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", supportModel),
		attribute.String("gen_ai.response.model", supportModel),
		attribute.String("gen_ai.conversation.id", req.ConversationId),
		attribute.String("user.id", req.UserId),
		attribute.Int64("gen_ai.usage.input_tokens", promptTokens),
		attribute.Int64("gen_ai.usage.output_tokens", outputTokens),
		attribute.Float64("gen_ai.usage.input_cost", float64(promptTokens)*0.15/1_000_000),
		attribute.Float64("gen_ai.usage.output_cost", float64(outputTokens)*0.60/1_000_000),
		attribute.String("gen_ai.response.finish_reason", finishReason),
		attribute.String("gen_ai.prompt", string(promptJSON)),
		attribute.String("gen_ai.completion", completion),
	)
	time.Sleep(time.Duration(400+rand.IntN(700)) * time.Millisecond)
	span.End()
}

func runLookupOrderTool(c *gin.Context, orderId string) string {
	ctx := c.Request.Context()
	_, span := tracer.Start(ctx, "tool lookup_order", trace.WithAttributes(
		attribute.String("tool.name", "lookup_order"),
		attribute.String("tool.args", fmt.Sprintf(`{"order_id":"%s"}`, orderId)),
	))
	time.Sleep(time.Duration(80+rand.IntN(150)) * time.Millisecond)
	span.End()
	return fmt.Sprintf(`{"order_id":"%s","status":"shipped","carrier":"DHL","eta_days":2}`, orderId)
}

func assistantCompletion(reply string) string {
	b, _ := json.Marshal(gin.H{"choices": []gin.H{{
		"message":       gin.H{"role": "assistant", "content": reply},
		"finish_reason": "stop",
	}}})
	return string(b)
}

func extractOrderId(message string) string {
	upper := strings.ToUpper(message)
	idx := strings.Index(upper, "ORD-")
	if idx < 0 {
		return "ORD-000000"
	}
	end := idx + 4
	for end < len(upper) && upper[end] >= '0' && upper[end] <= '9' {
		end++
	}
	return upper[idx:end]
}
