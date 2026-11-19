package ops

import (
	"bytes"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Invocations parses a shell command line and returns the words of every
// simple command in it, with quoting removed ("a b" and 'a b' become a b).
// Words that need expansion ($VAR, $(...)) keep their source text.
func Invocations(cmdline string) ([][]string, error) {
	file, err := syntax.NewParser().Parse(strings.NewReader(cmdline), "")
	if err != nil {
		return nil, err
	}
	var out [][]string
	syntax.Walk(file, func(n syntax.Node) bool {
		ce, ok := n.(*syntax.CallExpr)
		if !ok || len(ce.Args) == 0 {
			return true
		}
		words := make([]string, 0, len(ce.Args))
		for _, w := range ce.Args {
			words = append(words, WordText(w))
		}
		out = append(out, words)
		return true
	})
	return out, nil
}

// WordText is a word's value with quotes and backslash escapes removed.
func WordText(w *syntax.Word) string {
	var b strings.Builder
	for _, p := range w.Parts {
		b.WriteString(partText(p, false))
	}
	return b.String()
}

func partText(p syntax.WordPart, inDouble bool) string {
	switch t := p.(type) {
	case *syntax.Lit:
		return unescape(t.Value, inDouble)
	case *syntax.SglQuoted:
		return t.Value
	case *syntax.DblQuoted:
		var b strings.Builder
		for _, q := range t.Parts {
			b.WriteString(partText(q, true))
		}
		return b.String()
	default:
		var buf bytes.Buffer
		_ = syntax.NewPrinter().Print(&buf, p)
		return buf.String()
	}
}

func unescape(s string, inDouble bool) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if !inDouble || strings.IndexByte("\"\\$`\n", next) >= 0 {
				if next != '\n' {
					b.WriteByte(next)
				}
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
