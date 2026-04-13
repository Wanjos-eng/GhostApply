// Package db fornece uma conexão SQLite criptografada com SQLCipher (AES-256).
//
// # Intenção
// Centraliza o acesso ao banco para que nenhum outro pacote abra conexão bruta.
// A chave de criptografia é aplicada pelo DSN antes de qualquer SQL ser executado.
//
// # Restrição (SecOps)
// A chave fica embutida no DSN e nunca é guardada em campo de struct.
// O chamador deve buscá-la no ambiente e descartá-la após Open.
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	// Driver CGO: exige CGO_ENABLED=1 e libsqlcipher-dev no ambiente de build.
	// O import em branco registra o driver "sqlite3" no database/sql.
	_ "github.com/mattn/go-sqlite3"
)

// Open retorna uma conexão SQLite criptografada com AES-256 via SQLCipher.
//
// A chave entra pelo mecanismo `_pragma`, então vira a primeira operação
// executada pelo driver, equivalente ao `PRAGMA key` do Rust.
//
// `path` pode ser `:memory:` em testes; SQLCipher aceita chave vazia nesse caso.
func Open(path, key string) (*sql.DB, error) {
	baseDSN := buildSQLiteDSN(path, "")
	trimmedKey := strings.TrimSpace(key)

	if trimmedKey == "" {
		return openWithDSN(path, baseDSN)
	}

	dsnWithKey := buildSQLiteDSN(path, trimmedKey)
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

func buildSQLiteDSN(path, key string) string {
	// O driver sqlite3 requer prefixo 'file:' e barras '/' (ToSlash) para processar os pragmas
	// sem tratar o '?' como um caractere inválido no nome do arquivo do Windows C:\
	normalizedPath := filepath.ToSlash(path)
	trimmedKey := strings.TrimSpace(key)

	if trimmedKey == "" {
		return fmt.Sprintf(
			"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
			normalizedPath,
		)
	}

	escapedKey := strings.ReplaceAll(trimmedKey, "'", "''")
	return fmt.Sprintf(
		"file:%s?_pragma=key('%s')&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		normalizedPath,
		escapedKey,
	)
}

func openWithDSN(path, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: failed to open '%s': %w", path, err)
	}

	// Ping força o driver a abrir o arquivo e aplicar os pragmas de fato.
	// Sem isso, erros só apareceriam na primeira consulta.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.Open: failed to verify connection (wrong key?): %w", err)
	}

	return db, nil
}
