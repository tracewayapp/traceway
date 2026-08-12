package contentflag

import (
	"reflect"
	"strings"
	"testing"
)

func TestScanFindsBuiltinTerms(t *testing.T) {
	m := NewMatcher(nil, nil)
	got := m.Scan("well this is complete bullshit and I hate it")
	want := []string{"bullshit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanIsCaseInsensitive(t *testing.T) {
	m := NewMatcher(nil, nil)
	got := m.Scan("What the FUCK", "this is Shit")
	want := []string{"fuck", "shit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanWordBoundaries(t *testing.T) {
	m := NewMatcher(nil, nil)
	// None of these contain a flagged term as a whole token.
	got := m.Scan(
		"the Scunthorpe assessment covered classic hancock analysis",
		"the shipment passed inspection",
		"a cocktail of assumptions",
	)
	if got != nil {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestScanPhrases(t *testing.T) {
	m := NewMatcher(nil, nil)
	got := m.Scan("you son of a bitch, I'm in")
	want := []string{"bitch", "son of a bitch"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanCustomTerms(t *testing.T) {
	m := NewMatcher(nil, []string{"acmecorp", "Secret Project"})
	got := m.Scan("mentioning AcmeCorp and the secret project here")
	want := []string{"acmecorp", "secret project"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestScanCustomDoesNotReplaceBuiltins(t *testing.T) {
	m := NewMatcher(nil, []string{"acmecorp"})
	got := m.Scan("acmecorp is shit")
	want := []string{"acmecorp", "shit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLanguagePacks(t *testing.T) {
	spanish := NewMatcher([]string{"es"}, nil)
	if got := spanish.Scan("esto es una mierda, hijo de puta"); !reflect.DeepEqual(got, []string{"hijo de puta", "mierda", "puta"}) {
		t.Errorf("es pack: got %v", got)
	}
	// The Spanish pack must not flag English profanity.
	if got := spanish.Scan("this is bullshit"); got != nil {
		t.Errorf("es pack should not match English terms, got %v", got)
	}

	german := NewMatcher([]string{"de"}, nil)
	if got := german.Scan("so eine Scheiße, du Arschloch"); !reflect.DeepEqual(got, []string{"arschloch", "scheiße"}) {
		t.Errorf("de pack: got %v", got)
	}

	serbian := NewMatcher([]string{"sr"}, nil)
	if got := serbian.Scan("kakvo sranje, jebote"); !reflect.DeepEqual(got, []string{"jebote", "sranje"}) {
		t.Errorf("sr pack: got %v", got)
	}

	multi := NewMatcher([]string{"en", "es", "de"}, nil)
	got := multi.Scan("bullshit y mierda und Scheiße")
	if !reflect.DeepEqual(got, []string{"bullshit", "mierda", "scheiße"}) {
		t.Errorf("multi pack: got %v", got)
	}
}

func TestExplicitEmptyLanguagesMeansCustomOnly(t *testing.T) {
	m := NewMatcher([]string{}, []string{"acmecorp"})
	if got := m.Scan("this is bullshit about acmecorp"); !reflect.DeepEqual(got, []string{"acmecorp"}) {
		t.Errorf("expected custom-only match, got %v", got)
	}
}

func TestUnknownLanguageIgnored(t *testing.T) {
	m := NewMatcher([]string{"xx", "en"}, nil)
	if got := m.Scan("this is bullshit"); !reflect.DeepEqual(got, []string{"bullshit"}) {
		t.Errorf("unknown pack code should be ignored, got %v", got)
	}
}

func TestAvailableLanguages(t *testing.T) {
	langs := AvailableLanguages()
	want := []string{"de", "en", "es", "fr", "it", "pt", "sr"}
	if !reflect.DeepEqual(langs, want) {
		t.Errorf("AvailableLanguages = %v, want %v", langs, want)
	}
	if !IsValidLanguage("en") || IsValidLanguage("xx") {
		t.Error("IsValidLanguage misbehaving")
	}
}

func TestScanEmptyAndNil(t *testing.T) {
	m := NewMatcher(nil, nil)
	if got := m.Scan(); got != nil {
		t.Fatalf("expected nil for no texts, got %v", got)
	}
	if got := m.Scan("", ""); got != nil {
		t.Fatalf("expected nil for empty texts, got %v", got)
	}
	if got := m.Scan("a perfectly polite conversation"); got != nil {
		t.Fatalf("expected nil for clean text, got %v", got)
	}
}

func TestScanCapsMatches(t *testing.T) {
	terms := make([]string, 0, maxMatches+10)
	var text strings.Builder
	for i := 0; i < maxMatches+10; i++ {
		term := "customterm" + strings.Repeat("x", i+1)
		terms = append(terms, term)
		text.WriteString(term + " ")
	}
	m := NewMatcher(nil, terms)
	got := m.Scan(text.String())
	if len(got) > maxMatches {
		t.Fatalf("expected at most %d matches, got %d", maxMatches, len(got))
	}
}

func TestScanTruncatesLargeText(t *testing.T) {
	// The flagged term sits beyond the scan cap and must not be found.
	text := strings.Repeat("clean ", maxScanBytes/6+1) + " bullshit"
	m := NewMatcher(nil, nil)
	if got := m.Scan(text); got != nil {
		t.Fatalf("expected no matches beyond the scan cap, got %v", got)
	}
}

func TestNewMatcherSharedForSamePacks(t *testing.T) {
	if NewMatcher(nil, nil) != NewMatcher(nil, nil) {
		t.Fatal("expected the default matcher to be shared")
	}
	if NewMatcher([]string{"en"}, nil) != NewMatcher(nil, nil) {
		t.Fatal("expected explicit [en] to share the default matcher")
	}
	if NewMatcher([]string{"es", "de"}, nil) != NewMatcher([]string{"de", "es"}, nil) {
		t.Fatal("expected language order not to matter for the cache")
	}
}
