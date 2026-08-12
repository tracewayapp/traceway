package contentflag

import (
	"embed"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const (
	// maxScanBytes bounds per-text CPU on the ingest hot path; conversation
	// payloads can reach megabytes.
	maxScanBytes = 256 * 1024
	maxMatches   = 20
)

// Built-in language packs, one term per line. Deliberately conservative
// lists of common profanity; operators extend them per project via custom
// terms and pick which packs apply (projects.ai_flagged_languages).
// Word-boundary matching tokenizes on non-letter/digit runes, so packs only
// work for languages that separate words (no CJK).
//
//go:embed terms/*.txt
var termFiles embed.FS

// DefaultLanguages is what projects get when they have not chosen packs
// (and what the ingest falls back to when no project is resolvable).
var DefaultLanguages = []string{"en"}

var languagePacks = loadLanguagePacks()

func loadLanguagePacks() map[string][]string {
	packs := map[string][]string{}
	entries, err := termFiles.ReadDir("terms")
	if err != nil {
		panic("contentflag: embedded terms directory missing: " + err.Error())
	}
	for _, entry := range entries {
		lang := strings.TrimSuffix(entry.Name(), ".txt")
		data, err := termFiles.ReadFile("terms/" + entry.Name())
		if err != nil {
			panic("contentflag: reading " + entry.Name() + ": " + err.Error())
		}
		var terms []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				terms = append(terms, line)
			}
		}
		packs[lang] = terms
	}
	return packs
}

// AvailableLanguages returns the built-in pack codes, sorted.
func AvailableLanguages() []string {
	langs := make([]string, 0, len(languagePacks))
	for lang := range languagePacks {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// IsValidLanguage reports whether a built-in pack exists for the code.
func IsValidLanguage(code string) bool {
	_, ok := languagePacks[code]
	return ok
}

// Matcher scans text for flagged terms using whole-token matching, so a
// term never matches inside a larger word (no Scunthorpe-style false
// positives).
type Matcher struct {
	words        map[string]struct{}
	phrases      [][]string
	maxPhraseLen int
}

// packMatcherCache caches matchers for language-pack-only configurations
// (no custom terms), keyed by the canonical language list. The set of
// distinct configurations per process is tiny.
var packMatcherCache sync.Map

// NewMatcher returns a matcher over the selected language packs merged with
// the given per-project custom terms. nil languages means DefaultLanguages;
// an explicitly empty (non-nil) slice means no built-in packs, custom terms
// only. Unknown language codes are ignored.
func NewMatcher(languages []string, customTerms []string) *Matcher {
	if languages == nil {
		languages = DefaultLanguages
	}
	if len(customTerms) == 0 {
		key := canonicalLanguageKey(languages)
		if cached, ok := packMatcherCache.Load(key); ok {
			return cached.(*Matcher)
		}
		m := buildMatcher(languages, nil)
		packMatcherCache.Store(key, m)
		return m
	}
	return buildMatcher(languages, customTerms)
}

func canonicalLanguageKey(languages []string) string {
	sorted := append([]string(nil), languages...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func buildMatcher(languages []string, customTerms []string) *Matcher {
	m := &Matcher{words: make(map[string]struct{})}
	for _, lang := range languages {
		for _, term := range languagePacks[lang] {
			m.add(term)
		}
	}
	for _, term := range customTerms {
		m.add(term)
	}
	return m
}

func (m *Matcher) add(term string) {
	tokens := tokenize(term, 0)
	switch len(tokens) {
	case 0:
	case 1:
		m.words[tokens[0]] = struct{}{}
	default:
		m.phrases = append(m.phrases, tokens)
		if len(tokens) > m.maxPhraseLen {
			m.maxPhraseLen = len(tokens)
		}
	}
}

// Scan returns the sorted, deduplicated flagged terms found in the given
// texts, capped at maxMatches. Phrase terms are reported space-joined.
func (m *Matcher) Scan(texts ...string) []string {
	var found map[string]struct{}
	record := func(term string) {
		if found == nil {
			found = make(map[string]struct{})
		}
		found[term] = struct{}{}
	}

	for _, text := range texts {
		if len(found) >= maxMatches {
			break
		}
		if text == "" {
			continue
		}
		if len(text) > maxScanBytes {
			text = text[:maxScanBytes]
		}
		tokens := tokenize(text, maxScanBytes)
		for i, token := range tokens {
			if len(found) >= maxMatches {
				break
			}
			if _, ok := m.words[token]; ok {
				record(token)
			}
			for _, phrase := range m.phrases {
				if i+len(phrase) > len(tokens) {
					continue
				}
				matched := true
				for j, want := range phrase {
					if tokens[i+j] != want {
						matched = false
						break
					}
				}
				if matched {
					record(strings.Join(phrase, " "))
				}
			}
		}
	}

	if len(found) == 0 {
		return nil
	}
	terms := make([]string, 0, len(found))
	for term := range found {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	if len(terms) > maxMatches {
		terms = terms[:maxMatches]
	}
	return terms
}

// tokenize lowercases and splits on any rune that is not a letter or digit.
// A byte limit of 0 means no limit.
func tokenize(s string, limit int) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for i, r := range s {
		if limit > 0 && i >= limit {
			break
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			flush()
		}
	}
	flush()
	return tokens
}
