package sdkconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RolloutByInstance buckets clients by their OpAMP instance uid, which is stable per browser profile.
const RolloutByInstance = "instance"

const maxConditions = 20

var numericOps = map[string]bool{"gt": true, "gte": true, "lt": true, "lte": true}

// Condition targets a rule at clients by the attributes they report in their OpAMP agent description.
type Condition struct {
	Key    string `json:"key"`
	Op     string `json:"op"`
	Value  any    `json:"value,omitempty"`
	Values []any  `json:"values,omitempty"`
}

// Rollout limits a rule to a share of the clients its conditions match. The bucket is a hash of the rule id and the
// client (or the named attribute), so a client stays in or out as long as the percentage does not drop below it.
type Rollout struct {
	Percent float64 `json:"percent"`
	By      string  `json:"by,omitempty"`
}

// Conditions match when every condition holds and the client falls inside the rollout. Empty conditions match all.
type Conditions struct {
	All     []Condition `json:"all,omitempty"`
	Rollout *Rollout    `json:"rollout,omitempty"`
}

func ParseConditions(raw []byte) (Conditions, error) {
	var c Conditions
	if len(bytes.TrimSpace(raw)) == 0 {
		return c, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, invalid("conditions are not a valid document: %v", err)
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	c.canonicalize()
	return c, nil
}

func (c Conditions) Validate() error {
	if len(c.All) > maxConditions {
		return invalid("a rule may have at most %d conditions", maxConditions)
	}
	for i, cond := range c.All {
		if cond.Key == "" {
			return invalid("conditions[%d].key is required", i)
		}
		if err := validateCondition(cond.Key, cond.Op, cond.Value, cond.Values); err != nil {
			return invalid("conditions[%d]: %s", i, err.(*ValidationError).Message)
		}
		if _, ok := numberValue(cond.Value); numericOps[cond.Op] && !ok {
			return invalid("conditions[%d]: %s needs a number", i, cond.Op)
		}
	}
	if r := c.Rollout; r != nil {
		if r.Percent < 0 || r.Percent > 100 {
			return invalid("rollout.percent must be between 0 and 100")
		}
		if len(r.By) > maxConditionKeyLen {
			return invalid("rollout.by must be at most %d characters", maxConditionKeyLen)
		}
	}
	return nil
}

// canonicalize stores the value of a numeric comparison as a number. The dashboard form sends "120", and the
// stored document, so its config hash, should not depend on how the number was typed.
func (c *Conditions) canonicalize() {
	for i, cond := range c.All {
		if n, ok := numberValue(cond.Value); numericOps[cond.Op] && ok {
			c.All[i].Value = n
		}
	}
}

// Matches evaluates the conditions against a client's attributes. salt is what the rollout hashes with the client,
// normally the rule id, so different rules sample independent cohorts.
func (c Conditions) Matches(attrs map[string]string, instanceUid [16]byte, salt string) bool {
	for _, cond := range c.All {
		if !cond.matches(attrs) {
			return false
		}
	}
	if c.Rollout == nil {
		return true
	}
	subject := instanceUid[:]
	if by := c.Rollout.By; by != "" && by != RolloutByInstance {
		value, ok := attrs[by]
		if !ok || value == "" {
			return false
		}
		subject = []byte(value)
	}
	return rolloutBucket(salt, subject) < uint32(c.Rollout.Percent*100)
}

func rolloutBucket(salt string, subject []byte) uint32 {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte{0})
	h.Write(subject)
	return binary.BigEndian.Uint32(h.Sum(nil)[:4]) % 10000
}

func (cond Condition) matches(attrs map[string]string) bool {
	actual, present := attrs[cond.Key]
	present = present && actual != ""
	switch cond.Op {
	case "exists":
		return present
	case "not_exists":
		return !present
	case "eq":
		return present && same(actual, cond.Value)
	case "neq":
		return !present || !same(actual, cond.Value)
	case "in":
		return present && anySame(actual, cond.Values)
	case "not_in":
		return !present || !anySame(actual, cond.Values)
	case "contains":
		return present && strings.Contains(actual, scalarString(cond.Value))
	case "icontains":
		return present && strings.Contains(strings.ToLower(actual), strings.ToLower(scalarString(cond.Value)))
	case "starts_with":
		return present && strings.HasPrefix(actual, scalarString(cond.Value))
	case "glob":
		return present && GlobMatch(actual, scalarString(cond.Value))
	case "gt", "gte", "lt", "lte":
		a, aok := parseNumber(actual)
		e, eok := numberValue(cond.Value)
		if !present || !aok || !eok {
			return false
		}
		switch cond.Op {
		case "gt":
			return a > e
		case "gte":
			return a >= e
		case "lt":
			return a < e
		}
		return a <= e
	case "version":
		return present && SatisfiesVersion(actual, scalarString(cond.Value))
	}
	return false
}

func scalarString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func parseNumber(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil
}

// numberValue reads the expected value of a numeric comparison: a JSON number, or a string holding a finite one
// (JSON cannot encode the others).
func numberValue(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		f, ok := parseNumber(val)
		return f, ok && !math.IsInf(f, 0) && !math.IsNaN(f)
	}
	return 0, false
}

func same(actual string, expected any) bool {
	if n, ok := expected.(float64); ok {
		if a, aok := parseNumber(actual); aok {
			return a == n
		}
	}
	return actual == scalarString(expected)
}

func anySame(actual string, values []any) bool {
	for _, v := range values {
		if same(actual, v) {
			return true
		}
	}
	return false
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	end := 0
	for end < len(v) && (v[end] == '.' || (v[end] >= '0' && v[end] <= '9')) {
		end++
	}
	if end == 0 {
		return nil
	}
	var parts []int
	for _, p := range strings.Split(strings.Trim(v[:end], "."), ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		parts = append(parts, n)
	}
	return parts
}

// CompareVersions compares dotted numeric versions part by part, so 1.10 > 1.9 and four-part browser versions
// work. ok is false when either side has no leading number.
func CompareVersions(a, b string) (cmp int, ok bool) {
	pa, pb := versionParts(a), versionParts(b)
	if pa == nil || pb == nil {
		return 0, false
	}
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

// SatisfiesVersion checks a version against a constraint such as ">=2.1 <3": comparators separated by spaces or
// commas, all of which must hold. It is the same grammar the browser SDK evaluates for trigger conditions.
func SatisfiesVersion(version, constraint string) bool {
	comparators := strings.FieldsFunc(constraint, func(r rune) bool { return r == ' ' || r == ',' })
	if len(comparators) == 0 {
		return false
	}
	for _, c := range comparators {
		op := ""
		for _, candidate := range []string{">=", "<=", "!=", "==", ">", "<", "="} {
			if strings.HasPrefix(c, candidate) {
				op = candidate
				break
			}
		}
		cmp, ok := CompareVersions(version, strings.TrimPrefix(c, op))
		if !ok {
			return false
		}
		var holds bool
		switch op {
		case ">=":
			holds = cmp >= 0
		case "<=":
			holds = cmp <= 0
		case ">":
			holds = cmp > 0
		case "<":
			holds = cmp < 0
		case "!=":
			holds = cmp != 0
		default:
			holds = cmp == 0
		}
		if !holds {
			return false
		}
	}
	return true
}

// GlobMatch is a case-insensitive wildcard match (`*` any run, `?` one character), the same algorithm the browser
// SDK runs for trigger conditions. Greedy with one backtrack point, so it stays linear-ish and cannot blow up the
// way a backtracking regular expression can.
func GlobMatch(value, pattern string) bool {
	t := []rune(strings.ToLower(value))
	p := []rune(strings.ToLower(pattern))
	ti, pi, star, mark := 0, 0, -1, 0
	for ti < len(t) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == t[ti]):
			ti++
			pi++
		case pi < len(p) && p[pi] == '*':
			star = pi
			pi++
			mark = ti
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}
