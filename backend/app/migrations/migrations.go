package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

type ExtensionMigration struct {
	Source embed.FS
	Path   string
	Table  string
}

var ExtensionPostgresMigrations []ExtensionMigration

//go:embed sqlite/*.sql
var migrationsSqliteFS embed.FS

const sqliteTrackingDDL = `CREATE TABLE IF NOT EXISTS %s (
	version TEXT PRIMARY KEY,
	applied_at DATETIME DEFAULT (datetime('now'))
)`

func runMigrationsOn(target *sql.DB, fsys embed.FS, dir, trackingTable, createTrackingSQL string) error {
	if _, err := target.Exec(fmt.Sprintf(createTrackingSQL, trackingTable)); err != nil {
		return fmt.Errorf("failed to create %s table: %w", trackingTable, err)
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		version := strings.TrimSuffix(file, ".up.sql")

		var count int
		err := target.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE version = ?", trackingTable), version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration version %s: %w", version, err)
		}
		if count > 0 {
			continue
		}

		content, err := fsys.ReadFile(dir + "/" + file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		statements := splitStatements(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := target.Exec(stmt); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", file, err)
			}
		}

		if _, err := target.Exec(fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", trackingTable), version); err != nil {
			return fmt.Errorf("failed to record migration version %s: %w", version, err)
		}
	}

	return nil
}

func splitStatements(content string) []string {
	var statements []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		switch {
		case inSingle:
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(content) && content[i+1] == '\'' {
					current.WriteByte(content[i+1])
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			current.WriteByte(ch)
			if ch == '"' {
				if i+1 < len(content) && content[i+1] == '"' {
					current.WriteByte(content[i+1])
					i++
				} else {
					inDouble = false
				}
			}
		case ch == '\'':
			inSingle = true
			current.WriteByte(ch)
		case ch == '"':
			inDouble = true
			current.WriteByte(ch)
		case ch == ';':
			statements = append(statements, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	statements = append(statements, current.String())
	return statements
}
