package workspace

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrSecretInDiff = errors.New("the diff contains what looks like a credential")

// secretPatterns are the credential shapes a pull request must never carry.
// They are deliberately few and specific: a false positive blocks a fix, a
// false negative ships a key.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`twp_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`xox[abprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
}

// SecretScan refuses a patch whose added lines carry a credential.
func SecretScan(patch string) error {
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		for _, pattern := range secretPatterns {
			if pattern.MatchString(line) {
				return fmt.Errorf("%w: a line matching %s", ErrSecretInDiff, pattern.String())
			}
		}
	}
	return nil
}
