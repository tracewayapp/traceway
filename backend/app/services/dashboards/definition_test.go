package dashboards

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUniqueName(t *testing.T) {
	takenOf := func(names ...string) func(string) bool {
		return func(name string) bool {
			for _, n := range names {
				if strings.EqualFold(n, name) {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		name    string
		taken   []string
		base    string
		project string
		want    string
	}{
		{"free", nil, "Mobile Performance", "Android App", "Mobile Performance"},
		{"taken names the project", []string{"Mobile Performance"}, "Mobile Performance", "Android App", "Mobile Performance · Android App"},
		{"case-insensitive", []string{"mobile performance"}, "Mobile Performance", "iOS App", "Mobile Performance · iOS App"},
		{"project copy taken too", []string{"Mobile Performance", "Mobile Performance · Android App"}, "Mobile Performance", "Android App", "Mobile Performance · Android App 2"},
		{"no project numbers it", []string{"Web Performance"}, "Web Performance", " ", "Web Performance 2"},
		{"numbers skip taken ones", []string{"Web Performance", "Web Performance 2"}, "Web Performance", "", "Web Performance 3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := UniqueName(takenOf(c.taken...), c.base, c.project, 100); got != c.want {
				t.Errorf("UniqueName = %q, want %q", got, c.want)
			}
		})
	}
}

func TestUniqueNameStaysWithinTheLimit(t *testing.T) {
	project := strings.Repeat("é", 120)
	taken := map[string]bool{"Mobile Performance": true}
	got := UniqueName(func(name string) bool { return taken[name] }, "Mobile Performance", project, 100)
	if utf8.RuneCountInString(got) != 100 || !strings.HasPrefix(got, "Mobile Performance · é") {
		t.Fatalf("UniqueName = %q (%d runes), want the template name and as much of the project name as fits in 100", got, utf8.RuneCountInString(got))
	}
	taken[got] = true
	second := UniqueName(func(name string) bool { return taken[name] }, "Mobile Performance", project, 100)
	if utf8.RuneCountInString(second) != 100 || !strings.HasSuffix(second, " 2") {
		t.Fatalf("second copy = %q (%d runes), want a numbered name within 100 runes", second, utf8.RuneCountInString(second))
	}
}
