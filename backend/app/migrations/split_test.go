package migrations

import (
	"strings"
	"testing"
)

func nonEmptyStatements(statements []string) []string {
	var out []string
	for _, s := range statements {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func TestSplitStatementsKeepsSemicolonsInsideStringLiterals(t *testing.T) {
	content := `INSERT INTO dashboard_templates (key, definition) VALUES ('demo', '{"widgets":[{"title":"a;b","config":{"query":"x;y;z"}}]}')`
	statements := nonEmptyStatements(splitStatements(content))
	if len(statements) != 1 {
		t.Fatalf("expected 1 statement, got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[0], `"x;y;z"`) {
		t.Errorf("string literal was mangled: %q", statements[0])
	}
}

func TestSplitStatementsHandlesEscapedSingleQuotes(t *testing.T) {
	content := `INSERT INTO t (s) VALUES ('it''s; still one');
UPDATE t SET s = 'done'`
	statements := nonEmptyStatements(splitStatements(content))
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[0], "it''s; still one") {
		t.Errorf("escaped quote literal was mangled: %q", statements[0])
	}
	if !strings.HasPrefix(statements[1], "UPDATE") {
		t.Errorf("second statement is wrong: %q", statements[1])
	}
}

func TestSplitStatementsHandlesQuotedIdentifiers(t *testing.T) {
	content := `CREATE TABLE "weird;name" (id INTEGER);
SELECT 1`
	statements := nonEmptyStatements(splitStatements(content))
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[0], `"weird;name"`) {
		t.Errorf("quoted identifier was mangled: %q", statements[0])
	}
}

func TestSplitStatementsSplitsPlainStatements(t *testing.T) {
	content := `CREATE TABLE a (id INTEGER PRIMARY KEY);

CREATE INDEX idx_a ON a(id);
`
	statements := nonEmptyStatements(splitStatements(content))
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(statements), statements)
	}
	if !strings.HasPrefix(statements[0], "CREATE TABLE") || !strings.HasPrefix(statements[1], "CREATE INDEX") {
		t.Errorf("statements split incorrectly: %#v", statements)
	}
}
