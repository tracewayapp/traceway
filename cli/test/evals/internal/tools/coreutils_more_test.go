package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoreUtilities(t *testing.T) {
	s, work := newShell(t)
	os.WriteFile(filepath.Join(work, "Notes.MD"), []byte("x\n"), 0o644)
	os.Symlink("notes.txt", filepath.Join(work, "link.txt"))
	cases := []struct{ cmd, want string }{
		{`sed -n '2p' notes.txt`, "beta 1"},
		{`sed 's/alpha/ALPHA/' notes.txt | head -1`, "ALPHA 3"},
		{`echo 'beta beta beta' | sed 's/beta/B/2'`, "beta B beta"},
		{`echo 'beta beta beta' | sed 's/beta/B/2g'`, "beta B B"},
		{`echo 'a x/y' | sed -e 's/a/b/;s/x\/y/z/'`, "b z"},
		{`find . -iname 'notes.md'`, "./Notes.MD"},
		{`find . -type l`, "./link.txt"},
		{`find . -maxdepth 1 -type f -name 'link*'`, ""},
		{`seq 3 | xargs echo`, "1 2 3"},
		{`date -u -d '2026-01-02T03:04:05Z' +%Y-%m-%d`, "2026-01-02"},
	}
	for _, c := range cases {
		r := run(t, s, c.cmd)
		if !r.OK || strings.TrimSpace(r.Output) != c.want {
			t.Errorf("%s: got ok=%v %q, want %q", c.cmd, r.OK, r.Output, c.want)
		}
	}
}
