// Package db fornece uma conexão SQLite local com driver puro Go.
//
// # Intenção
// Centraliza o acesso ao banco para que nenhum outro pacote abra conexão bruta.
// O driver é puro Go para evitar dependência de CGO nos builds Windows.
//
// # Restrição (SecOps)
// O chamador deve buscar a configuração no ambiente e descartá-la após Open.
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	// Driver puro Go para evitar dependência de CGO nos builds Windows.
	_ "github.com/glebarez/go-sqlite"
)

// Open retorna uma conexão SQLite local com pragmas mínimos habilitados.
//
// O parâmetro de chave é mantido por compatibilidade com a API existente,
// mas o driver usado não implementa criptografia nativa.
//
// `path` pode ser `:memory:` em testes.
func Open(path, key string) (*sql.DB, error) {
	baseDSN := buildSQLiteDSN(path)
	trimmedKey := strings.TrimSpace(key)

	if trimmedKey == "" {
		return openWithDSN(path, baseDSN)
	}

	dsnWithKey := buildSQLiteDSN(path)
	db, err := openWithDSN(path, dsnWithKey)
	if err == nil {
		return db, nil
	}

	plainDB, plainErr := openWithDSN(path, baseDSN)
	if plainErr == nil {
		return plainDB, nil
	}

	return nil, fmt.Errorf("db.Open: with key failed: %v; plain fallback failed: %v", err, plainErr)
}

func buildSQLiteDSN(path string) string {
	// O driver puro Go aceita URI file: com barras normalizadas.
	normalizedPath := filepath.ToSlash(path)
	return fmt.Sprintf("file:%s", normalizedPath)
}

func openWithDSN(path, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: failed to open '%s': %w", path, err)
	}

	// Ping força o driver a abrir o arquivo e aplicar os pragmas de fato.
	// Sem isso, erros só apareceriam na primeira consulta.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.Open: failed to verify connection (wrong key?): %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.Open: failed to enable foreign keys: %w", err)
	}
	if path != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("db.Open: failed to enable WAL: %w", err)
		}
	}

	return db, nil
}
