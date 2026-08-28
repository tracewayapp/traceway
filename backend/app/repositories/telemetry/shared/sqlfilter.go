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

// MethodPrefix is the uppercase "GET " form every backend compares against.
// The stored method is whatever the instrumented service reported --
// getHTTPEndpoint concatenates http.request.method verbatim -- so the column
// side is upper-cased at query time rather than the value being trusted.
func MethodPrefix(method string) string {
	return strings.ToUpper(method) + " "
}

// MethodFilterClause matches the method case-insensitively. SUBSTR rather than
// LIKE because methodFilter reaches SQL as a value and LIKE would read % and _
// in it as wildcards; UPPER around the column rather than a second bound
// parameter so the comparison cannot depend on how the caller cased its input.
// UPPER is ASCII-only in SQLite, which is exactly the range RFC 9110 allows a
// method token.
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
