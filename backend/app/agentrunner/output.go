package agentrunner

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

var credentialPattern = regexp.MustCompile(`(?:sk-ant-[A-Za-z0-9_-]+|gh[pousr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|xox[baprs]-[A-Za-z0-9-]+|twp_[A-Za-z0-9_-]+|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`)

func redactOutput(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
		encoded, _ := json.Marshal(secret)
		value = strings.ReplaceAll(value, string(encoded[1:len(encoded)-1]), "[REDACTED]")
	}
	return credentialPattern.ReplaceAllString(value, "[REDACTED]")
}

type tailBuffer struct{ bytes.Buffer }

func (b *tailBuffer) Write(p []byte) (int, error) {
	const limit = 8192
	n := len(p)
	if len(p) > limit {
		p = p[len(p)-limit:]
	}
	if b.Len()+len(p) > limit {
		b.Next(b.Len() + len(p) - limit)
	}
	_, err := b.Buffer.Write(p)
	return n, err
}
