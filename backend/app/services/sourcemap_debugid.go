package services

import (
	"encoding/json"
	"regexp"
	"strings"
)

const sourceMapDebugIdDir = "by-debug-id/"

var debugIdRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var bundleDebugIdCommentRe = regexp.MustCompile(`//[#@] debugId=([0-9a-fA-F-]{36})`)

func NormalizeDebugId(raw string) string {
	id := strings.ToLower(strings.TrimSpace(raw))
	if debugIdRe.MatchString(id) {
		return id
	}
	return ""
}

func DebugIdMapName(debugId string) string {
	return sourceMapDebugIdDir + debugId + ".js.map"
}

func DebugIdBundleName(debugId string) string {
	return sourceMapDebugIdDir + debugId + ".js"
}

func ExtractDebugId(fileName string, data []byte) string {
	if strings.HasSuffix(fileName, ".map") {
		var fields struct {
			DebugId       string `json:"debugId"`
			LegacyDebugId string `json:"debug_id"`
		}
		if err := json.Unmarshal(data, &fields); err != nil {
			return ""
		}
		if id := NormalizeDebugId(fields.DebugId); id != "" {
			return id
		}
		return NormalizeDebugId(fields.LegacyDebugId)
	}

	tail := data
	if len(tail) > 4096 {
		tail = tail[len(tail)-4096:]
	}
	matches := bundleDebugIdCommentRe.FindAllSubmatch(tail, -1)
	if len(matches) == 0 {
		return ""
	}
	return NormalizeDebugId(string(matches[len(matches)-1][1]))
}
