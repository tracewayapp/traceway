//go:build transactional_pg

package db

func initMainDB() error {
	return initPostgres()
}
