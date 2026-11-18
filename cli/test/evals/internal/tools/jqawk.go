package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	awkinterp "github.com/benhoyt/goawk/interp"
	awkparser "github.com/benhoyt/goawk/parser"
	"github.com/itchyny/gojq"
	"mvdan.cc/sh/v3/interp"
)

// runJQ implements jq with gojq, in-process.
func runJQ(hc interp.HandlerContext, r *shellRun, args []string) int {
	u := &uctx{hc: hc, run: r}
	raw, compact, slurp, nullIn, exitStatus, sortKeys, rawIn, join := false, false, false, false, false, false, false, false
	indent := 2
	var filter string
	haveFilter := false
	var files []string
	var varNames []string
	var varValues []any
	// Options may come before or after the filter, as in jq; "--" ends them.
	optionsDone := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case optionsDone || a == "-" || !strings.HasPrefix(a, "-"):
			if !haveFilter {
				filter, haveFilter = a, true
			} else {
				files = append(files, a)
			}
		case a == "--":
			optionsDone = true
		case a == "--arg" && i+2 < len(args):
			varNames = append(varNames, "$"+args[i+1])
			varValues = append(varValues, args[i+2])
			i += 2
		case a == "--argjson" && i+2 < len(args):
			var v any
			if err := json.Unmarshal([]byte(args[i+2]), &v); err != nil {
				u.errf("jq: --argjson %s: invalid JSON", args[i+1])
				return 2
			}
			varNames = append(varNames, "$"+args[i+1])
			varValues = append(varValues, v)
			i += 2
		case a == "--tab":
			indent = -1
		case a == "--indent" && i+1 < len(args):
			fmt.Sscanf(args[i+1], "%d", &indent)
			i++
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--raw-output":
				raw = true
			case "--compact-output":
				compact = true
			case "--slurp":
				slurp = true
			case "--null-input":
				nullIn = true
			case "--exit-status":
				exitStatus = true
			case "--sort-keys":
				sortKeys = true
			case "--raw-input":
				rawIn = true
			case "--join-output":
				raw, join = true, true
			case "--monochrome-output", "--color-output":
			default:
				u.errf("jq: unsupported option %s", a)
				return 2
			}
		default:
			for _, c := range a[1:] {
				switch c {
				case 'r':
					raw = true
				case 'j':
					raw, join = true, true
				case 'c':
					compact = true
				case 's':
					slurp = true
				case 'n':
					nullIn = true
				case 'e':
					exitStatus = true
				case 'S':
					sortKeys = true
				case 'R':
					rawIn = true
				case 'M', 'C', 'a':
				default:
					u.errf("jq: unsupported option -%c", c)
					return 2
				}
			}
		}
	}
	if !haveFilter {
		filter = "."
	}
	_ = sortKeys
	query, err := gojq.Parse(filter)
	if err != nil {
		u.errf("jq: error: %v", err)
		return 3
	}
	code, err := gojq.Compile(query, gojq.WithVariables(varNames), gojq.WithEnvironLoader(func() []string { return nil }))
	if err != nil {
		u.errf("jq: error: %v", err)
		return 3
	}
	data, rc := u.inputs(files)
	if rc != 0 {
		return 2
	}
	var inputs []any
	switch {
	case nullIn:
		inputs = []any{nil}
	case rawIn:
		lines := splitLines(data)
		if slurp {
			inputs = []any{string(data)}
		} else {
			for _, l := range lines {
				inputs = append(inputs, l)
			}
		}
	default:
		dec := json.NewDecoder(bytes.NewReader(data))
		var all []any
		for {
			var v any
			if err := dec.Decode(&v); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				u.errf("jq: error (at <stdin>): %v", err)
				return 2
			}
			all = append(all, normalizeJSON(v))
		}
		if slurp {
			if all == nil {
				all = []any{}
			}
			inputs = []any{all}
		} else {
			inputs = all
		}
	}
	var out bytes.Buffer
	last := any(nil)
	produced := false
	status := 0
	for _, in := range inputs {
		iter := code.Run(in, varValues...)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				var halt *gojq.HaltError
				if errors.As(err, &halt) {
					if halt.Value() != nil {
						u.errf("jq: error: %v", halt.Value())
					}
					hc.Stdout.Write(out.Bytes())
					return halt.ExitCode()
				}
				u.errf("jq: error: %v", err)
				status = 5
				break
			}
			produced, last = true, v
			if s, ok := v.(string); ok && raw {
				out.WriteString(s)
			} else {
				out.Write(encodeJSON(v, compact, indent))
			}
			if !join {
				out.WriteByte('\n')
			}
		}
	}
	hc.Stdout.Write(out.Bytes())
	if status != 0 {
		return status
	}
	if exitStatus && (!produced || last == nil || last == false) {
		return 1
	}
	return 0
}

// normalizeJSON converts encoding/json values into the types gojq expects.
func normalizeJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = normalizeJSON(x)
		}
		return t
	case []any:
		for i, x := range t {
			t[i] = normalizeJSON(x)
		}
		return t
	case float64:
		if t == float64(int64(t)) && t < 1<<53 && t > -(1<<53) {
			return int(t)
		}
		return t
	default:
		return v
	}
}

func encodeJSON(v any, compact bool, indent int) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if !compact {
		if indent < 0 {
			enc.SetIndent("", "\t")
		} else if indent > 0 {
			enc.SetIndent("", strings.Repeat(" ", indent))
		}
	}
	if err := enc.Encode(v); err != nil {
		return []byte(fmt.Sprint(v))
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// runAWK implements awk with goawk: no exec, no file writes; file operands
// are read through the sandbox.
func runAWK(hc interp.HandlerContext, r *shellRun, args []string) int {
	u := &uctx{hc: hc, run: r}
	var vars []string
	var src string
	haveSrc := false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-F" && i+1 < len(args):
			vars = append(vars, "FS", unescapeAwk(args[i+1]))
			i++
		case strings.HasPrefix(a, "-F") && len(a) > 2:
			vars = append(vars, "FS", unescapeAwk(a[2:]))
		case a == "-v" && i+1 < len(args):
			k, v, _ := strings.Cut(args[i+1], "=")
			vars = append(vars, k, v)
			i++
		case a == "-f":
			u.errf("awk: -f is not supported; pass the program inline")
			return 2
		default:
			if !haveSrc {
				src, haveSrc = a, true
			} else {
				files = append(files, a)
			}
		}
	}
	if !haveSrc {
		u.errf("awk: no program")
		return 2
	}
	prog, err := awkparser.ParseProgram([]byte(src), nil)
	if err != nil {
		u.errf("awk: %v", err)
		return 2
	}
	data, rc := u.inputs(files)
	if rc != 0 {
		return 2
	}
	status, err := awkinterp.ExecProgram(prog, &awkinterp.Config{
		Stdin: bytes.NewReader(data), Output: hc.Stdout, Error: hc.Stderr,
		Vars: vars, NoExec: true, NoFileWrites: true, NoFileReads: true, Environ: []string{},
	})
	if err != nil {
		u.errf("awk: %v", err)
		return 2
	}
	return status
}

func unescapeAwk(s string) string {
	if s == "\\t" || s == "t" {
		return "\t"
	}
	return s
}
