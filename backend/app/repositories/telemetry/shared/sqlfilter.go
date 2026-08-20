package shared

import "sort"

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
func MethodFilterClause(qualifiedCol, methodFilter string) string {
	if methodFilter != "" {
		return " AND " + qualifiedCol + " LIKE :method"
	}
	return ""
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
