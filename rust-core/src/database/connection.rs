//! Fábrica de conexões com o banco SQLCipher.
//!
//! # Intenção
//! Abrir uma conexão SQLite **e** ativar a criptografia AES-256 de forma atômica,
//! garantindo que nenhuma operação ocorra antes do `PRAGMA key`.
//!
//! # Restrição (SecOps — Tarefa 06)
//! O `PRAGMA key` deve ser a **primeira** instrução executada após `open`.
//! Qualquer operação antes dele receberá os dados como texto puro — falha de segurança crítica.

use anyhow::{Context, Result};
use rusqlite::Connection;

/// Abre uma conexão SQLite criptografada com SQLCipher (AES-256).
///
/// # Parâmetros
/// - `db_path`: caminho para o arquivo `.sqlite`, ou `":memory:"` para testes.
/// - `key`: chave mestra AES-256. Será consumida por `PRAGMA key` e não precisa
///   ser armazenada após essa chamada — o chamador deve `zeroize` o valor original.
///
/// # Segurança
/// Os `PRAGMA`s são executados na seguinte ordem obrigatória:
/// 1. `PRAGMA key` — ativa a criptografia; **nenhuma leitura/escrita antes disto**.
/// 2. `PRAGMA journal_mode = WAL` — performance; seguro pós-chave.
/// 3. `PRAGMA foreign_keys = ON` — integridade referencial; só funciona com chave ativa.
pub fn open_connection(db_path: &str, key: &str) -> Result<Connection> {
    let conn = Connection::open(db_path)
        .with_context(|| format!("falha ao abrir o banco em '{db_path}'"))?;

    // ── SecOps: PRAGMA key DEVE ser o primeiro comando ────────────────────────
    // Sanitiza a chave escapando aspas simples (SQLite escape: ' → '').
    // Sem isso, uma chave contendo `'` permite SQL injection arbitrário.
    if !key.is_empty() {
        let sanitized_key = key.replace('\'', "''");
        conn.execute_batch(&format!("PRAGMA key = '{sanitized_key}';"))
            .context("falha ao aplicar PRAGMA key — verifique DB_ENCRYPTION_KEY")?;
    }

    // WAL permite leituras concorrentes sem travar escritas
    conn.execute_batch("PRAGMA journal_mode = WAL;")
        .context("falha ao configurar journal_mode")?;

    // FKs são desativadas por padrão no SQLite; habilitar por conexão é mandatório
    conn.execute_batch("PRAGMA foreign_keys = ON;")
        .context("falha ao habilitar foreign_keys")?;

    Ok(conn)
}
