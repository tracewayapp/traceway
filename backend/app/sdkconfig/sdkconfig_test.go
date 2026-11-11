package sdkconfig

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustSettings(t *testing.T, raw string) Settings {
	t.Helper()
	s, err := ParseSettings([]byte(raw))
	if err != nil {
		t.Fatalf("ParseSettings(%s): %v", raw, err)
	}
	return s
}

func mustConditions(t *testing.T, raw string) Conditions {
	t.Helper()
	c, err := ParseConditions([]byte(raw))
	if err != nil {
		t.Fatalf("ParseConditions(%s): %v", raw, err)
	}
	return c
}

func uid(s string) [16]byte {
	return [16]byte(uuid.MustParse(s))
}

func TestParseSettingsRejectsInvalidDocuments(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field":        `{"signals":{"errors":{"mode":"stream","loud":true}}}`,
		"bad mode":             `{"signals":{"console":{"mode":"sometimes"}}}`,
		"rate above one":       `{"replay":{"sessionSampleRate":1.5}}`,
		"baseline above one":   `{"sessions":{"baselineSampleRate":1.1}}`,
		"baseline negative":    `{"sessions":{"baselineSampleRate":-0.1}}`,
		"bad severity":         `{"signals":{"console":{"minSeverity":"chatty"}}}`,
		"wrong schema":         `{"schemaVersion":2}`,
		"captures in a rule":   `{"captures":[{"id":"c","until":"2026-01-01T00:00:00Z"}]}`,
		"buffer too big":       `{"buffer":{"maxBytes":999999999999}}`,
		"trigger bad op":       `{"triggers":[{"id":"t","steps":[{"match":{"key":"x","op":"like","value":"y"}}],"actions":[{"type":"flush","lookbackSeconds":5}]}]}`,
		"trigger two forms":    `{"triggers":[{"id":"t","steps":[{"match":{"key":"x","op":"eq","value":"y","all":[]}}],"actions":[{"type":"flush","lookbackSeconds":5}]}]}`,
		"trigger no lookback":  `{"triggers":[{"id":"t","steps":[{"match":{"key":"x","op":"exists"}}],"actions":[{"type":"flush"}]}]}`,
		"steps without action": `{"triggers":[{"id":"t","steps":[{"match":{"key":"x","op":"exists"}}]}]}`,
		"duplicate trigger":    `{"triggers":[{"id":"t","enabled":false},{"id":"t","enabled":true}]}`,
		"bad signal":           `{"triggers":[{"id":"t","steps":[{"match":{"key":"x","op":"exists"}}],"actions":[{"type":"stream","durationSeconds":5,"signals":["everything"]}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSettings([]byte(raw))
			if _, ok := err.(*ValidationError); !ok {
				t.Fatalf("got %v, want a ValidationError", err)
			}
		})
	}
	if _, err := ParseSettings([]byte(strings.Repeat(" ", MaxDocumentBytes+1))); err == nil {
		t.Fatal("oversized document accepted")
	}
}

func TestEvaluateLaysMatchingRulesInOrder(t *testing.T) {
	compiled := &Compiled{Rules: []Rule{
		{Id: "all", Settings: mustSettings(t, `{"signals":{"network":{"mode":"stream","sampleRate":0.5}},"triggers":[{"id":"default-exception","enabled":false}]}`)},
		{Id: "old-chrome", Conditions: mustConditions(t, `{"all":[{"key":"browser.name","op":"eq","value":"Chrome"},{"key":"browser.version","op":"version","value":"<120"}]}`),
			Settings: mustSettings(t, `{"signals":{"network":{"mode":"off"}},"replay":{"mode":"stream"}}`)},
		{Id: "release", Conditions: mustConditions(t, `{"all":[{"key":"service.version","op":"in","values":["2.3.1","2.3.2"]}]}`),
			Settings: mustSettings(t, `{"signals":{"console":{"mode":"stream","minSeverity":"warn"}}}`)},
	}}
	someone := uid("0195f2a0-0000-7000-8000-000000000001")

	got := compiled.Evaluate(map[string]string{"browser.name": "Chrome", "browser.version": "119.0.6045.105", "service.version": "2.3.1"}, someone, time.Now())
	if strings.Join(got.MatchedRules, ",") != "all,old-chrome,release" {
		t.Fatalf("matched %v", got.MatchedRules)
	}
	s := got.Settings
	if *s.Signals.Network.Mode != ModeOff || *s.Signals.Network.SampleRate != 0.5 {
		t.Errorf("later rules override only what they set: %+v", s.Signals.Network)
	}
	if *s.Replay.Mode != ModeStream || *s.Signals.Console.MinSeverity != "warn" {
		t.Errorf("unexpected overlay: %+v", s)
	}
	if len(s.Triggers) != 1 || *s.Triggers[0].Enabled {
		t.Errorf("trigger patch not carried: %+v", s.Triggers)
	}

	firefox := compiled.Evaluate(map[string]string{"browser.name": "Firefox", "service.version": "1.0.0"}, someone, time.Now())
	if strings.Join(firefox.MatchedRules, ",") != "all" || *firefox.Settings.Signals.Network.Mode != ModeStream {
		t.Errorf("unexpected result for a client the targeted rules miss: %+v", firefox)
	}

	if compiled.Rules[0].Settings.Signals.Network.Mode == nil || *compiled.Rules[0].Settings.Signals.Network.Mode != ModeStream {
		t.Error("evaluation must not modify the compiled rules")
	}
}

func TestRolloutIsStickyAndProportional(t *testing.T) {
	cond := mustConditions(t, `{"rollout":{"percent":10}}`)
	wider := mustConditions(t, `{"rollout":{"percent":30}}`)
	in := 0
	for i := 0; i < 100000; i++ {
		u := [16]byte(uuid.New())
		if cond.Matches(nil, u, "rule-a") {
			in++
			if !wider.Matches(nil, u, "rule-a") {
				t.Fatal("raising the percentage must keep every client that was already in")
			}
		}
	}
	if in < 9500 || in > 10500 {
		t.Fatalf("10%% rollout matched %d of 100000", in)
	}

	byUser := mustConditions(t, `{"rollout":{"percent":100,"by":"user.id"}}`)
	if byUser.Matches(map[string]string{}, [16]byte{}, "r") {
		t.Error("a rollout by attribute must not match clients without it")
	}
}

func TestConditionOperators(t *testing.T) {
	attrs := map[string]string{"service.version": "2.10.0", "user.id": "u-7", "plan": "pro", "seats": "12"}
	cases := map[string]bool{
		`{"key":"service.version","op":"version","value":">=2.9 <3"}`: true,
		`{"key":"service.version","op":"version","value":">2.10"}`:    false,
		`{"key":"plan","op":"neq","value":"free"}`:                    true,
		`{"key":"missing","op":"neq","value":"x"}`:                    true,
		`{"key":"plan","op":"not_in","values":["free","trial"]}`:      true,
		`{"key":"user.id","op":"contains","value":"u-"}`:              true,
		`{"key":"seats","op":"gte","value":10}`:                       true,
		`{"key":"seats","op":"eq","value":12}`:                        true,
		`{"key":"missing","op":"exists"}`:                             false,
		`{"key":"missing","op":"not_exists"}`:                         true,
	}
	for raw, want := range cases {
		c := mustConditions(t, `{"all":[`+raw+`]}`)
		if got := c.Matches(attrs, [16]byte{}, "r"); got != want {
			t.Errorf("%s = %v, want %v", raw, got, want)
		}
	}
}

func TestEvaluateAddsActiveCaptures(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	lookback := 600.0
	compiled := &Compiled{Captures: []ActiveCapture{
		{Id: "c-device", Conditions: mustConditions(t, `{"all":[{"key":"service.instance.id","op":"eq","value":"dev-1"}]}`), ExpiresAt: now.Add(time.Hour), LookbackSeconds: &lookback},
		{Id: "c-expired", ExpiresAt: now.Add(-time.Second)},
	}}
	got := compiled.Evaluate(map[string]string{"service.instance.id": "dev-1"}, [16]byte{}, now)
	if len(got.Settings.Captures) != 1 || got.Settings.Captures[0].Until != "2026-11-01T13:00:00Z" {
		t.Fatalf("unexpected captures: %+v", got.Settings.Captures)
	}
	other := compiled.Evaluate(map[string]string{"service.instance.id": "dev-2"}, [16]byte{}, now)
	if len(other.Settings.Captures) != 0 {
		t.Fatalf("capture leaked to another device")
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	s := mustSettings(t, `{"triggers":[{"id":"t","steps":[{"match":{"all":[{"key":"event.name","op":"eq","value":"exception"}]}}],"actions":[{"type":"emit","eventName":"x","attributes":{"b":"2","a":"1"}}]}],"signals":{"errors":{"mode":"buffer"}}}`)
	body1, hash1, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	reparsed := mustSettings(t, string(body1))
	body2, hash2, _ := Render(reparsed)
	if string(body1) != string(body2) || string(hash1) != string(hash2) {
		t.Fatalf("render not stable:\n%s\n%s", body1, body2)
	}
	empty, _, _ := Render(Settings{})
	if string(empty) != `{"schemaVersion":1}` {
		t.Errorf("empty overlay = %s", empty)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body1, &decoded); err != nil || decoded["schemaVersion"].(float64) != 1 {
		t.Errorf("rendered document is not the SDK's schema: %s", body1)
	}
}

func TestApplyPermissionSwitchesOffWhatThePlanDrops(t *testing.T) {
	s := mustSettings(t, `{"signals":{"errors":{"mode":"stream"},"console":{"mode":"stream"}},"replay":{"mode":"stream"}}`)
	ApplyPermission(&s, false, true, false)
	if *s.Signals.Errors.Mode != ModeOff || *s.Signals.Messages.Mode != ModeOff || *s.Replay.Mode != ModeOff {
		t.Errorf("exceptions/replay not switched off: %+v", s)
	}
	if *s.Signals.Console.Mode != ModeStream {
		t.Errorf("data signals must stay as configured")
	}
	if s.Limits == nil || *s.Limits.Replay != ModeOff || s.Limits.Signals["errors"] != ModeOff ||
		s.Limits.Signals["messages"] != ModeOff || len(s.Limits.Signals) != 2 {
		t.Errorf("limits = %+v, want errors, messages and replay off", s.Limits)
	}
	body, _, err := Render(s)
	if err != nil || !strings.Contains(string(body), `"limits":{"signals":{"errors":"off","messages":"off"},"replay":"off"}`) {
		t.Errorf("rendered limits: %s %v", body, err)
	}
	if _, err := ParseSettings([]byte(`{"limits":{"replay":"off"}}`)); err == nil {
		t.Errorf("a rule must not be able to set limits")
	}
}

func TestLogLineTriggerValidation(t *testing.T) {
	valid := `{"triggers":[{"id":"payment-log","steps":[{"match":{"all":[{"key":"event.name","op":"eq","value":"browser.console"},{"key":"body","op":"glob","value":"*payment failed*"}]},"count":5,"periodSeconds":60}],"actions":[{"type":"capture","durationSeconds":1800,"lookbackSeconds":120,"replay":true}]}]}`
	s := mustSettings(t, valid)
	if *s.Triggers[0].Steps[0].Count != 5 || s.Triggers[0].Actions[0].Type != "capture" {
		t.Fatalf("unexpected parse: %+v", s.Triggers[0])
	}
	for name, raw := range map[string]string{
		"capture without duration": `{"triggers":[{"id":"t","steps":[{"match":{"key":"body","op":"icontains","value":"x"}}],"actions":[{"type":"capture"}]}]}`,
		"count zero":               `{"triggers":[{"id":"t","steps":[{"match":{"key":"body","op":"starts_with","value":"x"},"count":0}],"actions":[{"type":"capture","durationSeconds":60}]}]}`,
		"period without count":     `{"triggers":[{"id":"t","steps":[{"match":{"key":"body","op":"glob","value":"x*"},"periodSeconds":60}],"actions":[{"type":"capture","durationSeconds":60}]}]}`,
		"regex op":                 `{"triggers":[{"id":"t","steps":[{"match":{"key":"body","op":"regex","value":"x+"}}],"actions":[{"type":"capture","durationSeconds":60}]}]}`,
	} {
		if _, err := ParseSettings([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestGlobMatchAndTextOperators(t *testing.T) {
	cases := []struct {
		value, pattern string
		want           bool
	}{
		{"Payment FAILED for order 1042", "payment failed*", true},
		{"payment failed for order 1042", "*order ????", true},
		{"payment failed for order 1042", "*order ???", false},
		{"abc", "a*b*c*", true},
		{"abc", "a*d", false},
		{"", "*", true},
	}
	for _, c := range cases {
		if got := GlobMatch(c.value, c.pattern); got != c.want {
			t.Errorf("GlobMatch(%q, %q) = %v, want %v", c.value, c.pattern, got, c.want)
		}
	}
	attrs := map[string]string{"browser.name": "Chrome Mobile", "user.id": "Jane@Acme.test"}
	for raw, want := range map[string]bool{
		`{"key":"browser.name","op":"starts_with","value":"Chrome"}`: true,
		`{"key":"browser.name","op":"glob","value":"*mobile"}`:       true,
		`{"key":"user.id","op":"icontains","value":"@acme."}`:        true,
		`{"key":"user.id","op":"contains","value":"@acme."}`:         false,
	} {
		if got := mustConditions(t, `{"all":[`+raw+`]}`).Matches(attrs, [16]byte{}, "r"); got != want {
			t.Errorf("%s = %v, want %v", raw, got, want)
		}
	}
}

func TestBaselineOverlayPreservesLifecycle(t *testing.T) {
	settings := mustSettings(t, `{"sessions":{"enabled":true,"baselineSampleRate":0.01}}`)
	Merge(&settings, mustSettings(t, `{"sessions":{"baselineSampleRate":0}}`))
	if settings.Sessions.Enabled == nil || !*settings.Sessions.Enabled || settings.Sessions.BaselineSampleRate == nil || *settings.Sessions.BaselineSampleRate != 0 {
		t.Fatalf("unexpected session settings: %+v", settings.Sessions)
	}
}
