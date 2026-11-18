package agent

// Prompt caching. Agent loops resend the whole history every turn, so caching
// is most of the cost saving.
//
//   - Anthropic models (caching: anthropic) get explicit cache_control
//     breakpoints: one on the system message (the generic prompt plus any
//     preloaded skill; tools sit before it in the cached prefix) and one on
//     the most recent message, so each turn reads all earlier history from
//     cache. Anthropic checks earlier block boundaries for hits on its own, so
//     a single moving breakpoint is enough.
//   - OpenAI models (caching: automatic) cache any stable prefix. The system
//     prompt therefore carries no timestamps and tools keep a fixed order.

// applyCaching returns the messages to send for a caching mode, without
// mutating the conversation itself.
func applyCaching(mode string, msgs []Message) []Message {
	if mode != "anthropic" || len(msgs) == 0 {
		return msgs
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	mark := func(i int) {
		m := out[i]
		text, ok := m.Content.(string)
		if !ok || text == "" {
			return
		}
		m.Content = []Part{{Type: "text", Text: text, CacheControl: &CacheControl{Type: "ephemeral"}}}
		out[i] = m
	}
	if out[0].Role == "system" {
		mark(0)
	}
	if last := len(out) - 1; last > 0 && (out[last].Role == "tool" || out[last].Role == "user") {
		mark(last)
	}
	return out
}
