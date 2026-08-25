//go:build !telemetry_ch && !telemetry_firebolt

package notifications

import (
	"reflect"
	"strings"
	"testing"
)

func TestMatchFlaggedTerms(t *testing.T) {
	flagged := []string{"bullshit", "acmecorp"}

	if got := matchFlaggedTerms(buildTermFilter(nil), flagged); !reflect.DeepEqual(got, flagged) {
		t.Errorf("empty filter should pass all terms, got %v", got)
	}
	if got := matchFlaggedTerms(buildTermFilter([]string{" AcmeCorp "}), flagged); !reflect.DeepEqual(got, []string{"acmecorp"}) {
		t.Errorf("filter should normalize and match acmecorp, got %v", got)
	}
	if got := matchFlaggedTerms(buildTermFilter([]string{"unrelated"}), flagged); got != nil {
		t.Errorf("non-matching filter should return nil, got %v", got)
	}
	if got := matchFlaggedTerms(buildTermFilter([]string{"", "  "}), flagged); !reflect.DeepEqual(got, flagged) {
		t.Errorf("filter of blank terms behaves as empty, got %v", got)
	}
}

func TestBuildAiConversationCostMessage(t *testing.T) {
	warning := buildAiConversationCostMessage("conv-1", 6.0, 5.0, "myproject")
	if warning.Severity != SeverityWarning {
		t.Errorf("expected warning severity at 1.2x threshold, got %v", warning.Severity)
	}
	if !strings.Contains(warning.Body, "conv-1") {
		t.Errorf("body %q should mention the conversation id", warning.Body)
	}
	if warning.DedupToken != "conv-1" {
		t.Errorf("dedup token = %q, expected conv-1", warning.DedupToken)
	}

	critical := buildAiConversationCostMessage("conv-1", 15.0, 5.0, "myproject")
	if critical.Severity != SeverityCritical {
		t.Errorf("expected critical severity at 3x threshold, got %v", critical.Severity)
	}
}

func TestBuildAiFlaggedContentMessage(t *testing.T) {
	msg := buildAiFlaggedContentMessage("conv-9", "user-7", []string{"bullshit"}, "myproject")
	if !strings.Contains(msg.Subject, "bullshit") {
		t.Errorf("subject %q should list matched terms", msg.Subject)
	}
	if !strings.Contains(msg.Body, "conv-9") || !strings.Contains(msg.Body, "user-7") {
		t.Errorf("body %q should mention conversation and user", msg.Body)
	}
	if !strings.Contains(msg.URL, "flagged=1") {
		t.Errorf("url %q should deep-link to the flagged filter", msg.URL)
	}

	anonymous := buildAiFlaggedContentMessage("", "", []string{"crap"}, "myproject")
	if !strings.Contains(anonymous.Body, "An AI conversation matched") {
		t.Errorf("body %q should fall back to the generic phrasing", anonymous.Body)
	}
}

func TestAiDedupKeysAreDistinct(t *testing.T) {
	convCost := aiConversationCostDedupKey(7, "abc")
	flaggedKey := aiFlaggedContentDedupKey(7, "abc")
	traceCost := aiCostDedupKey(7, "abc")
	if convCost == flaggedKey || convCost == traceCost || flaggedKey == traceCost {
		t.Errorf("dedup keys must not collide: %q %q %q", convCost, flaggedKey, traceCost)
	}
	if !strings.HasPrefix(convCost, ruleStatePrefix(7)) || !strings.HasPrefix(flaggedKey, ruleStatePrefix(7)) {
		t.Error("dedup keys must carry the rule-state prefix so ClearRuleState purges them")
	}
}
