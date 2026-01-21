package pgdb

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() error {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	database := os.Getenv("POSTGRES_DATABASE")
	username := os.Getenv("POSTGRES_USERNAME")
	password := os.Getenv("POSTGRES_PASSWORD")
	sslMode := os.Getenv("POSTGRES_SSLMODE")

	if sslMode == "" {
		sslMode = "disable"
	}
	if port == "" {
		port = "5432"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, username, password, database, sslMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	DB = db

	return nil
}

func GetDB() *sql.DB {
	return DB
}

func ExecuteTransaction[T any](f func(tx *sql.Tx) (T, error)) (T, error) {
	tx, err := DB.Begin()

	if err != nil {
		var zero T
		return zero, err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	result, err := f(tx)

	if err != nil {
		tx.Rollback()
		var zero T
		return zero, err
	}

	if err := tx.Commit(); err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}
