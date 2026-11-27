package envfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Result string

const (
	ResultCreated   Result = "created"
	ResultAdded     Result = "added"
	ResultUpdated   Result = "updated"
	ResultUnchanged Result = "unchanged"
)

// Upsert sets key to value in the env file at path, creating the file and its directory when missing. The file
// type follows the extension: an Xcode .xcconfig keeps "//" as "/$()/" (a bare "//" starts a comment there), and a
// .properties file also takes "key: value" lines.
func Upsert(path, key, value string) (Result, error) {
	return UpsertEquivalent(path, key, value, nil)
}

// UpsertEquivalent is Upsert that leaves the file alone when key already holds a value equivalent reports true for,
// such as an older spelling of the same connection string.
func UpsertEquivalent(path, key, value string, equivalent func(existing string) bool) (Result, error) {
	content, mode, existed, err := readEnvFile(path)
	if err != nil {
		return "", err
	}

	if equivalent != nil {
		if existing, found := lookupIn(path, content, key); found && equivalent(existing) {
			return ResultUnchanged, nil
		}
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	assignRe, err := assignmentRegex(path, key)
	if err != nil {
		return "", err
	}
	written := escapeValue(path, value)

	lines := strings.Split(content, "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSuffix(line, "\r")
		match := assignRe.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		suffix := ""
		if strings.HasSuffix(line, "\r") {
			suffix = "\r"
		}
		lines[i] = match[1] + key + "=" + written + suffix
		replaced = true
	}

	var updated string
	result := ResultUpdated
	if replaced {
		updated = strings.Join(lines, "\n")
	} else {
		updated = content
		if updated != "" && !strings.HasSuffix(updated, "\n") {
			updated += eol
		}
		updated += key + "=" + written + eol
		result = ResultAdded
		if !existed {
			result = ResultCreated
		}
	}

	if existed && updated == content {
		return ResultUnchanged, nil
	}

	if err := writeAtomic(path, updated, mode); err != nil {
		return "", err
	}
	return result, nil
}

// Lookup returns the value key is assigned in the env file at path (the last assignment, unquoted and unescaped),
// and whether it is assigned at all. A missing file has no keys.
func Lookup(path, key string) (string, bool, error) {
	content, _, existed, err := readEnvFile(path)
	if err != nil || !existed {
		return "", false, err
	}
	value, found := lookupIn(path, content, key)
	return value, found, nil
}

func lookupIn(path, content, key string) (string, bool) {
	assignRe, err := assignmentRegex(path, key)
	if err != nil {
		return "", false
	}
	value, found := "", false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSuffix(line, "\r")
		match := assignRe.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		value, found = unquote(strings.TrimSpace(line[match[1]:])), true
	}
	if isXcconfig(path) {
		value = strings.ReplaceAll(value, "$()", "")
	}
	return value, found
}

func assignmentRegex(path, key string) (*regexp.Regexp, error) {
	separator := `=`
	if strings.HasSuffix(strings.ToLower(path), ".properties") {
		separator = `[=:]`
	}
	return regexp.Compile(`^(\s*(?:export\s+)?)` + regexp.QuoteMeta(key) + `\s*` + separator)
}

func isXcconfig(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".xcconfig")
}

func escapeValue(path, value string) string {
	if isXcconfig(path) {
		return strings.ReplaceAll(value, "//", "/$()/")
	}
	return value
}

func unquote(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && last == first {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func readEnvFile(path string) (content string, mode fs.FileMode, existed bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0o600, false, nil
	}
	if err != nil {
		return "", 0, false, fmt.Errorf("reading %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false, fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), info.Mode().Perm(), true, nil
}

func writeAtomic(path, content string, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-env-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
