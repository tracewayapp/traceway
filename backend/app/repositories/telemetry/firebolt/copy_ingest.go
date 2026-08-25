//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
)

// Bulk ingest via the engine's file loader: batches are written as CSV to a
// directory shared with the engine and loaded with
// INSERT INTO ... SELECT * FROM read_csv(...) — a native columnar load,
// orders of magnitude faster than SQL-text INSERTs on large batches.
// FIREBOLT_COPY_DIR is where the backend writes; FIREBOLT_COPY_DIR_ENGINE is
// the same directory as the engine sees it (defaults to the former). Unset
// FIREBOLT_COPY_DIR disables the path entirely. Any failure falls back to
// the literal INSERT path, so this is an optimization, never a correctness
// dependency.
const copyMinRows = 256

// Bare (unquoted) marker for SQL NULL. Every real value is quoted, so the
// marker cannot collide with data.
const copyNullMarker = `\N`

var copyFileSeq atomic.Uint64

func copyDirs() (local string, engine string, ok bool) {
	local = strings.TrimSpace(config.Config.FireboltCopyDir)
	if local == "" {
		return "", "", false
	}
	engine = strings.TrimSpace(config.Config.FireboltCopyDirEngine)
	if engine == "" {
		engine = local
	}
	return local, engine, true
}

func renderCSVField(sb *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		sb.WriteString(copyNullMarker)
	case string:
		sb.WriteByte('"')
		sb.WriteString(strings.ReplaceAll(x, `"`, `""`))
		sb.WriteByte('"')
	case int64:
		sb.WriteString(strconv.FormatInt(x, 10))
	case int:
		sb.WriteString(strconv.Itoa(x))
	case uint32:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case float64:
		sb.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case time.Time:
		sb.WriteByte('"')
		sb.WriteString(x.UTC().Format("2006-01-02 15:04:05.000000"))
		sb.WriteByte('"')
	default:
		return fmt.Errorf("unsupported csv type %T", v)
	}
	return nil
}

func copyIngest(ctx context.Context, table string, columns []string, rows [][]any) error {
	localDir, engineDir, ok := copyDirs()
	if !ok {
		return fmt.Errorf("copy dir not configured")
	}

	var sb strings.Builder
	sb.Grow(len(rows) * len(columns) * 24)
	for _, row := range rows {
		for j, v := range row {
			if j > 0 {
				sb.WriteByte(',')
			}
			if err := renderCSVField(&sb, v); err != nil {
				return err
			}
		}
		sb.WriteByte('\n')
	}

	name := fmt.Sprintf("tw-%s-%d-%d.csv", table, time.Now().UnixNano(), copyFileSeq.Add(1))
	localPath := filepath.Join(localDir, name)
	if err := os.WriteFile(localPath, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(localPath)

	enginePath := engineDir + "/" + name
	query := "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") SELECT * FROM read_csv('file://" + enginePath +
		"', header => false, null_string => '" + copyNullMarker + "', empty_field_as_null => false)"
	if _, err := db.TelemetryDB.ExecContext(ctx, query); err != nil {
		return err
	}
	return nil
}
