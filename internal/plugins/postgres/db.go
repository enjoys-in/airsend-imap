package plugins

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type DB struct {
	Conn *sql.DB
}

// NewDB creates and verifies a PostgreSQL connection.
func CreateDBConnection(host, port, user, password, dbname string, sslmode string) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{Conn: db}, nil
}

func (d *DB) Close() error {
	log.Printf("Database Connection has been Closed")
	return d.Conn.Close()
}
