package shared

import (
	"fmt"
	"sort"
	"strings"
)

// RootFilterClause returns the WHERE fragment for the issues/endpoints
// root-vs-non-root filter; both embedded backends store is_root as 0/1.
func RootFilterClause(qualifiedCol, rootFilter string) string {
	switch rootFilter {
	case "root":
		return " AND " + qualifiedCol + " = 1"
	case "non_root":
		return " AND " + qualifiedCol + " = 0"
	default:
		return ""
	}
}

func MethodPrefix(method string) string {
	return strings.ToUpper(method) + " "
}

// UPPER on the column side so stored lowercase methods still match; SUBSTR rather than LIKE so % and _ in the value stay literal.
func MethodFilterClause(qualifiedCol, method string) (clause string, param string) {
	if method == "" {
		return "", ""
	}
	prefix := MethodPrefix(method)
	return fmt.Sprintf(" AND UPPER(SUBSTR(%s, 1, %d)) = :method", qualifiedCol, len(prefix)), prefix
}

// SortedKeys returns map keys in stable order so generated SQL and its
// bound parameters line up deterministically.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TraceIdsFilter matches the supplied trace IDs and writes its named parameters into params.
func TraceIdsFilter(traceIds []string, params map[string]any) string {
	names := make([]string, len(traceIds))
	for i, id := range traceIds {
		key := fmt.Sprintf("trace_%d", i)
		names[i] = ":" + key
		params[key] = id
	}
	list := strings.Join(names, ",")
	return "trace_id IN (" + list + ")"
}
