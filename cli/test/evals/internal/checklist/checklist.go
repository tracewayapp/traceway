// Package checklist turns the bottleneck checklist in performance.md into the
// enum submit_perf_diagnosis takes.
package checklist

import (
	"regexp"
	"strings"
	"sync"

	"github.com/tracewayapp/traceway/cli/v2/pkg/mcpserver/knowledge"
)

// Row is one row of the bottleneck checklist in performance.md.
type Row struct {
	ID       string // slug, the enum value submit_perf_diagnosis takes
	Title    string // row title as written, markdown stripped
	Category string // the ### heading above the table
	Confirm  string // the "How to confirm in Traceway" cell
}

// Extra answer values beyond the checklist rows.
const (
	NoneFound            = "none_found"
	InsufficientEvidence = "insufficient_evidence"
)

var (
	checklistOnce sync.Once
	checklist     []Row
)

// Rows parses the canonical performance.md once. The enum therefore
// tracks the skill: renaming a row renames its id, and the case test fails.
func Rows() []Row {
	checklistOnce.Do(func() { checklist = ParseChecklist(knowledge.MustRead("performance.md")) })
	return checklist
}

var (
	rowRe        = regexp.MustCompile(`^\|\s*\*\*(.+?)\*\*\s*\|\s*(.+?)\s*\|\s*$`)
	parenRe      = regexp.MustCompile(`\s*\([^)]*\s[^)]*\)`)
	nonAlnumRe   = regexp.MustCompile(`[^a-z0-9]+`)
	backtickRe   = regexp.MustCompile("`")
	headingRe    = regexp.MustCompile(`^###\s+(.+?)\s*$`)
	checklistTop = "## Bottleneck checklist"
)

// ParseChecklist extracts the checklist rows from a performance.md body.
func ParseChecklist(md string) []Row {
	var rows []Row
	in := false
	category := ""
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, checklistTop)
			continue
		}
		if !in {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			category = m[1]
			continue
		}
		m := rowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		title := backtickRe.ReplaceAllString(m[1], "")
		rows = append(rows, Row{ID: Slug(title), Title: title, Category: category, Confirm: m[2]})
	}
	return rows
}

// Slug turns a checklist title into its id: explanatory parentheticals (ones
// with a space inside) dropped, "+" spelled out, "^" dropped, everything else
// non-alphanumeric collapsed to "_".
func Slug(title string) string {
	s := parenRe.ReplaceAllString(title, "")
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "+", " plus ")
	s = strings.ReplaceAll(s, "^", "")
	s = nonAlnumRe.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// ChecklistIDs returns every valid checklist_item value, rows first.
func IDs() []string {
	var ids []string
	for _, r := range Rows() {
		ids = append(ids, r.ID)
	}
	return append(ids, NoneFound, InsufficientEvidence)
}

// IsChecklistID reports whether id is a valid checklist_item.
func IsID(id string) bool {
	for _, v := range IDs() {
		if v == id {
			return true
		}
	}
	return false
}
