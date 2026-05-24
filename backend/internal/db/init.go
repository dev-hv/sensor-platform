package db

import (
	"database/sql"
	"fmt"
	"strings"

	"sensor-backend/internal/schema"

	_ "github.com/lib/pq"
)

// InitDB connects to Postgres and creates the table from schema if not exists.
func InitDB(connStr string, s *schema.Schema) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := createTable(db, s); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return db, nil
}

func createTable(db *sql.DB, s *schema.Schema) error {
	var parts []string
	for _, c := range s.MandatoryColumns {
		parts = append(parts, fmt.Sprintf("%s %s", quoteIdent(c.Name), c.SQLType))
	}
	for _, c := range s.DynamicColumns {
		parts = append(parts, fmt.Sprintf("%s %s", quoteIdent(c.Name), c.SQLType))
	}
	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)",
		quoteIdent(s.TableName), strings.Join(parts, ", "))
	_, err := db.Exec(ddl)
	return err
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
