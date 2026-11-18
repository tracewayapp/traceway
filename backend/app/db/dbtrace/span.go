package dbtrace

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const maxQueryTextBytes = 2048

func StartSpan(ctx context.Context, opts Options, query string) (trace.Span, bool) {
	parent, ok := traceway.StatementParent(ctx)
	if !ok {
		return nil, false
	}
	return startSpan(parent, opts, query), true
}

func EndSpan(span trace.Span, err error) {
	recordError(span, err)
	span.End()
}

// Named "{operation} {table}" as the OpenTelemetry database conventions ask,
// so a waterfall reads "SELECT endpoints_v2" rather than a column of SELECTs.
func startSpan(parent context.Context, opts Options, query string, extra ...trace.SpanStartOption) trace.Span {
	operation, collection := Summarize(query)
	name := operation
	attrs := []attribute.KeyValue{
		attribute.String("db.system.name", opts.System),
		attribute.String("db.namespace", opts.Namespace),
		attribute.String("db.operation.name", operation),
		attribute.String("db.query.text", truncate(query, maxQueryTextBytes)),
	}
	if collection != "" {
		name += " " + collection
		attrs = append(attrs, attribute.String("db.collection.name", collection))
	}
	options := append([]trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	}, extra...)
	_, span := traceway.Tracer().Start(parent, name, options...)
	return span
}

// Failed statements mark the span but are not recorded as exceptions:
// Traceway turns exceptions into issues, and most query errors (a unique
// violation answered with 422) are handled by the caller.
func recordError(span trace.Span, err error) {
	if err == nil || errors.Is(err, driver.ErrSkip) || errors.Is(err, context.Canceled) || errors.Is(err, sql.ErrNoRows) {
		return
	}
	span.SetStatus(codes.Error, err.Error())
}

// Summarize returns the statement's operation and the table it works on, or
// no table when that is not a single named table (a subquery, no FROM).
func Summarize(query string) (operation, collection string) {
	tokens := sqlTokens(query)
	if len(tokens) == 0 {
		return "QUERY", ""
	}
	first := strings.ToUpper(tokens[0].text)
	if first == "WITH" {
		for i, t := range tokens {
			if t.depth == 0 && isStatementKeyword(t.text) && i > 0 {
				return summarizeFrom(tokens[i:])
			}
		}
		return "WITH", ""
	}
	return summarizeFrom(tokens)
}

func summarizeFrom(tokens []sqlToken) (string, string) {
	operation := strings.ToUpper(tokens[0].text)
	switch operation {
	case "SELECT", "DELETE":
		return operation, tableAfter(tokens, "FROM")
	case "INSERT", "REPLACE":
		return operation, tableAfter(tokens, "INTO")
	case "UPDATE":
		if len(tokens) > 1 && strings.EqualFold(tokens[1].text, "OR") && len(tokens) > 3 {
			return operation, tableName(tokens[3])
		}
		if len(tokens) > 1 {
			return operation, tableName(tokens[1])
		}
	default:
		if len(tokens) > 2 && strings.EqualFold(tokens[1].text, "TABLE") {
			return operation, tableName(tokens[2])
		}
	}
	return operation, ""
}

func isStatementKeyword(word string) bool {
	switch strings.ToUpper(word) {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "REPLACE":
		return true
	}
	return false
}

// A FROM over a subquery names the table the subquery reads, so
// "SELECT count(*) FROM (SELECT * FROM spans_v2) t" is "SELECT spans_v2".
func tableAfter(tokens []sqlToken, keyword string) string {
	depth := 0
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.depth != depth || !strings.EqualFold(t.text, keyword) || i+1 >= len(tokens) {
			continue
		}
		if tokens[i+1].text != "(" {
			return tableName(tokens[i+1])
		}
		keyword, depth = "FROM", depth+1
		i++
	}
	return ""
}

func tableName(t sqlToken) string {
	if t.text == "(" {
		return ""
	}
	return strings.Trim(t.text, "\"`[]")
}

type sqlToken struct {
	text  string
	depth int
}

// sqlTokens splits a statement into words and parentheses, skipping string
// literals and comments, and records how deep in parentheses each word is.
func sqlTokens(query string) []sqlToken {
	var tokens []sqlToken
	depth := 0
	for i := 0; i < len(query); {
		c := query[i]
		switch {
		case c == '\'':
			i = skipQuoted(query, i, '\'')
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				return tokens
			}
			i += end + 4
		case c == '(':
			tokens = append(tokens, sqlToken{text: "(", depth: depth})
			depth++
			i++
		case c == ')':
			if depth > 0 {
				depth--
			}
			i++
		case c == '"' || c == '`':
			end := skipQuoted(query, i, c)
			tokens = append(tokens, sqlToken{text: query[i:end], depth: depth})
			i = end
		case isWordByte(c):
			start := i
			for i < len(query) && isWordByte(query[i]) {
				i++
			}
			tokens = append(tokens, sqlToken{text: query[start:i], depth: depth})
		default:
			i++
		}
	}
	return tokens
}

func skipQuoted(query string, start int, quote byte) int {
	for i := start + 1; i < len(query); i++ {
		if query[i] == quote {
			if i+1 < len(query) && query[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(query)
}

func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c >= 0x80 || unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
