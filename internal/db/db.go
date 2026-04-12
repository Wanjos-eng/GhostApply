// Package db fornece uma conexão SQLite criptografada com SQLCipher (AES-256).
//
// # Intenção
// Centraliza o acesso ao banco para que nenhum outro pacote abra conexão bruta.
// A chave de criptografia é aplicada pelo DSN antes de qualquer SQL ser executado.
//
// # Restrição (SecOps — Tarefa 14)
// A chave fica embutida no DSN e nunca é guardada em campo de struct.
// O chamador deve buscá-la no ambiente e descartá-la após Open.
package db

import (
	"database/sql"
	"fmt"

	// CGO driver — requires CGO_ENABLED=1 and libsqlcipher-dev at build time.
	// The blank import registers the "sqlite3" driver with database/sql.
	_ "github.com/mattn/go-sqlite3"
)

// Open retorna uma conexão SQLite criptografada com AES-256 via SQLCipher.
//
// A chave entra pelo mecanismo `_pragma`, então vira a primeira operação
// executada pelo driver, equivalente ao `PRAGMA key` do Rust.
//
// `path` pode ser `:memory:` em testes; SQLCipher aceita chave vazia nesse caso.
func Open(path, key string) (*sql.DB, error) {
	// `_pragma=key` codifica a senha do SQLCipher diretamente no DSN.
	// O driver aplica isso antes de qualquer leitura ou escrita.
	dsn := fmt.Sprintf(
		"%s?_pragma=key('%s')&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		path, key,
	)

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
