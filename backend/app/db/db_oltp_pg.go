//go:build oltp_pg

package db

func initMainDB() error {
	return initPostgres()
}
