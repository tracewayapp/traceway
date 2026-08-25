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

func Upsert(path, key, value string) (Result, error) {
	content, mode, existed, err := readEnvFile(path)
	if err != nil {
		return "", err
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	assignRe, err := regexp.Compile(`^(\s*(?:export\s+)?)` + regexp.QuoteMeta(key) + `\s*=`)
	if err != nil {
		return "", err
	}

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
		lines[i] = match[1] + key + "=" + value + suffix
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
		updated += key + "=" + value + eol
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
