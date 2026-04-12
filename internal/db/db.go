// Package db provides an encrypted SQLite connection using SQLCipher (AES-256).
//
// # Intent
// Centralise all database wiring so no other package ever opens a raw connection.
// The encryption key is applied via the DSN before *any* SQL runs.
//
// # Constraint (SecOps — Task 14)
// The key is embedded in the DSN string, never stored in a struct field.
// Callers must source it from the environment and discard it after calling Open.
package db

import (
	"database/sql"
	"fmt"

	// CGO driver — requires CGO_ENABLED=1 and libsqlcipher-dev at build time.
	// The blank import registers the "sqlite3" driver with database/sql.
	_ "github.com/mattn/go-sqlite3"
)

// Open returns an AES-256 encrypted SQLite connection via SQLCipher.
//
// The key is injected through the DSN `_pragma` mechanism so it is the very
// first operation the driver executes — equivalent to Rust's `PRAGMA key`.
//
// path may be ":memory:" for testing (SQLCipher accepts an empty key for in-memory DBs).
func Open(path, key string) (*sql.DB, error) {
	// _pragma=key encodes the SQLCipher passphrase directly in the DSN.
	// SQLCipher applies it before any page is read or written.
	dsn := fmt.Sprintf(
		"%s?_pragma=key('%s')&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		path, key,
	)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: failed to open '%s': %w", path, err)
	}

	// Ping forces the driver to actually open the file and apply the pragmas.
	// Without this, errors (wrong key, corrupt file) surface only at first query.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.Open: failed to verify connection (wrong key?): %w", err)
	}

	return db, nil
}
