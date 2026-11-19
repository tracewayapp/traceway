package grade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// lineBeforeEdit maps a line of the agent's edited copy of rel back to the
// line of the fixture file it came from. The fixture is the case's repo
// under testdata/repos; the edited copy is read from the workspace when
// there is one, else rebuilt from the recorded edit_file and write_file
// calls. It reports false when either side is unavailable, when a shell
// redirection wrote the file (its content was not recorded) or when the
// file was not edited at all.
func lineBeforeEdit(in *Input, rel string, line int) (int, bool) {
	if in == nil || in.Case == nil || in.EvalsRoot == "" || in.Case.Workspace.Repo == "" || line < 1 {
		return 0, false
	}
	raw, err := os.ReadFile(filepath.Join(in.EvalsRoot, "testdata", "repos", in.Case.Workspace.Repo, filepath.FromSlash(rel)))
	if err != nil {
		return 0, false
	}
	before := string(raw)
	var after string
	if in.Workspace != "" {
		now, err := os.ReadFile(filepath.Join(in.Workspace, filepath.FromSlash(rel)))
		if err != nil {
			return 0, false
		}
		after = string(now)
	} else {
		var ok bool
		if after, ok = replayEdits(in.Run, rel, before); !ok {
			return 0, false
		}
	}
	if after == before {
		return 0, false
	}
	return mapLineBack(before, after, line)
}

// replayEdits applies the run's successful edit_file and write_file calls
// on rel to base, in order, the way the tools did. A write through the shell
// (a redirection) has no recorded content, so the result is unknown.
func replayEdits(run *record.Run, rel, base string) (string, bool) {
	if run == nil {
		return "", false
	}
	text := base
	for _, c := range run.Calls {
		if c.Rejected {
			continue
		}
		for _, op := range c.Ops {
			if op.Op.Op != ops.OpEditFile && op.Op.Op != ops.OpWriteFile {
				continue
			}
			p, _ := op.Args["path"].(string)
			if !samePath(p, rel) {
				continue
			}
			if !op.OK {
				continue
			}
			switch c.Tool {
			case "edit_file":
				var in struct {
					Old        string `json:"old_string"`
					New        string `json:"new_string"`
					ReplaceAll bool   `json:"replace_all"`
				}
				if json.Unmarshal(c.Args, &in) != nil || in.Old == "" || !strings.Contains(text, in.Old) {
					return "", false
				}
				if in.ReplaceAll {
					text = strings.ReplaceAll(text, in.Old, in.New)
				} else {
					text = strings.Replace(text, in.Old, in.New, 1)
				}
			case "write_file":
				var in struct {
					Content string `json:"content"`
				}
				if json.Unmarshal(c.Args, &in) != nil {
					return "", false
				}
				text = in.Content
			default:
				return "", false
			}
		}
	}
	return text, true
}

// mapLineBack maps a 1-based line of after to the line of before it
// corresponds to, through a longest-common-subsequence alignment of the two
// files' lines. A line that exists in both maps to its match. A line the
// edit introduced or changed maps to where it sits between the nearest
// matched lines around it, clamped to that gap, so an inserted guard above
// a statement maps onto the statement's old neighbourhood.
func mapLineBack(before, after string, line int) (int, bool) {
	a := strings.Split(before, "\n")
	b := strings.Split(after, "\n")
	if line < 1 || line > len(b) {
		return 0, false
	}
	const maxLines = 6000
	if len(a) > maxLines || len(b) > maxLines {
		return 0, false
	}
	// dp[i][j] is the LCS length of a[i:] and b[j:].
	dp := make([][]int32, len(a)+1)
	for i := range dp {
		dp[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	// toBefore[j] is the 1-based before line matched to after line j+1, or 0.
	toBefore := make([]int, len(b))
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			toBefore[j] = i + 1
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	if m := toBefore[line-1]; m > 0 {
		return m, true
	}
	prevJ, prevI := 0, 0
	for j := line - 2; j >= 0; j-- {
		if toBefore[j] > 0 {
			prevJ, prevI = j+1, toBefore[j]
			break
		}
	}
	nextJ, nextI := len(b)+1, len(a)+1
	for j := line; j < len(b); j++ {
		if toBefore[j] > 0 {
			nextJ, nextI = j+1, toBefore[j]
			break
		}
	}
	// The unmatched after lines between prevJ and nextJ replace the before
	// lines between prevI and nextI; place the line proportionally in that
	// gap, and at least one past the previous match.
	gapB := nextJ - prevJ
	gapA := nextI - prevI
	off := (line - prevJ) * gapA / gapB
	if off < 1 {
		off = 1
	}
	if off > gapA-1 && gapA > 1 {
		off = gapA - 1
	}
	got := prevI + off
	if got < 1 {
		got = 1
	}
	if got > len(a) {
		got = len(a)
	}
	return got, true
}
