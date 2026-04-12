//! `ghost-apply-core` — Cofre Criptográfico e Pipeline de Candidaturas.
//!
//! # Arquitetura (Clean Arch)
//! ```text
//! lib.rs         ← ponto de entrada público (entidades + use-cases básicos)
//! config.rs      ← carregamento seguro do .env (AppConfig + Zeroize)
//! database/
//!   connection.rs ← fábrica de conexão SQLCipher AES-256
//!   migrations.rs ← DDL idempotente (3 tabelas)
//! ```

pub mod config;
pub mod database;

use anyhow::{bail, Context, Result};
use rusqlite::Connection;
use serde::{Deserialize, Serialize};

// ── Entidades de domínio ──────────────────────────────────────────────────────

/// Vaga de emprego capturada de um portal.
///
/// `id` deve ser um UUID v4 gerado pelo chamador.
/// `url` é a chave de idempotência: `INSERT OR IGNORE` usa esse campo.
#[derive(Debug, Serialize, Deserialize)]
pub struct Vaga {
    pub id: String,
    pub titulo: String,
    pub empresa: String,
    pub url: String,
    pub descricao: Option<String>,
}

/// Estados válidos de uma `Candidatura_Forjada`.
///
/// Implementa a máquina de estados do pipeline de candidaturas.
/// Transições permitidas:
/// - `RASCUNHO` → `ENVIADA`
/// - `ENVIADA`  → `CONFIRMADA` | `REJEITADA`
#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub enum CandidaturaStatus {
    Rascunho,
    Enviada,
    Confirmada,
    Rejeitada,
}

impl CandidaturaStatus {
    /// Converte para a string usada no banco de dados.
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Rascunho   => "RASCUNHO",
            Self::Enviada    => "ENVIADA",
            Self::Confirmada => "CONFIRMADA",
            Self::Rejeitada  => "REJEITADA",
        }
    }
}

// ── Task 10: insert_vaga ──────────────────────────────────────────────────────

/// Persiste uma vaga no banco usando `INSERT OR IGNORE`.
///
/// # Intent
/// Idempotente por design: duas chamadas com o mesmo `vaga.url` resultam em
/// exatamente uma linha — a segunda é silenciosamente descartada.
/// Isso é fundamental para scrapers que re-processam a mesma página.
pub fn insert_vaga(conn: &Connection, vaga: &Vaga) -> Result<()> {
    conn.execute(
        "INSERT OR IGNORE INTO Vaga_Prospectada (id, titulo, empresa, url, descricao)
         VALUES (?1, ?2, ?3, ?4, ?5)",
        rusqlite::params![
            vaga.id,
            vaga.titulo,
            vaga.empresa,
            vaga.url,
            vaga.descricao,
        ],
    )
    .with_context(|| format!("falha ao inserir vaga id={}", vaga.id))?;

    Ok(())
}

// ── Task 11: update_status_candidatura ───────────────────────────────────────

/// Transiciona o status de uma `Candidatura_Forjada`.
///
/// # Constraint
/// Retorna `Err` se a candidatura não existir — o chamador deve tratar o caso
/// de ID inválido; não silenciamos falhas de atualização (zero rows affected).
pub fn update_status_candidatura(
    conn: &Connection,
    id: &str,
    status: CandidaturaStatus,
) -> Result<()> {
    let rows_affected = conn
        .execute(
            "UPDATE Candidatura_Forjada SET status = ?1 WHERE id = ?2",
            rusqlite::params![status.as_str(), id],
        )
        .with_context(|| format!("falha ao atualizar status da candidatura id={id}"))?;

    if rows_affected == 0 {
        bail!("candidatura id={id} não encontrada — nenhuma linha atualizada");
    }

    Ok(())
}

// ── Task 12: testes unitários ─────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use crate::database::{connection::open_connection, migrations::run_migrations};

    /// Abre um banco em memória SEM criptografia (SQLCipher aceita chave vazia em :memory:).
    fn banco_em_memoria() -> Connection {
        // Chave vazia é aceita pelo SQLCipher para bancos :memory: em testes
        open_connection(":memory:", "").expect("falha ao abrir banco em memória")
    }

    #[test]
    fn test_migrations_ssobem_sem_erro() {
        let conn = banco_em_memoria();
        run_migrations(&conn).expect("migrations devem rodar sem erros");
    }

    #[test]
    fn test_insert_vaga_idempotente() {
        let conn = banco_em_memoria();
        run_migrations(&conn).unwrap();

        let vaga = Vaga {
            id:       "uuid-001".to_string(),
            titulo:   "Engenheiro de Software".to_string(),
            empresa:  "GhostCorp".to_string(),
            url:      "https://linkedin.com/jobs/1".to_string(),
            descricao: Some("Vaga remota".to_string()),
        };

        // Primeira inserção: deve funcionar
        insert_vaga(&conn, &vaga).expect("primeira inserção deve funcionar");

        // Segunda inserção com mesma URL: INSERT OR IGNORE — sem erro, sem duplicata
        insert_vaga(&conn, &vaga).expect("segunda inserção deve ser silenciosa (OR IGNORE)");

        let count: i64 = conn
            .query_row("SELECT COUNT(*) FROM Vaga_Prospectada", [], |r| r.get(0))
            .unwrap();
        assert_eq!(count, 1, "OR IGNORE deve garantir exatamente 1 linha");
    }

    #[test]
    fn test_banco_memoria_isolamento_fk_invalida() {
        // # Intent (Task 12)
        // Validar que o banco rejeita Candidatura_Forjada com vaga_id inexistente.
        // Isso prova que PRAGMA foreign_keys = ON está ativo + schema está correto.
        let conn = banco_em_memoria();
        run_migrations(&conn).unwrap();

        let resultado = conn.execute(
            "INSERT INTO Candidatura_Forjada (id, vaga_id) VALUES ('c-001', 'vaga-inexistente')",
            [],
        );

        assert!(
            resultado.is_err(),
            "deve rejeitar candidatura com vaga_id inválido (FK violation)"
        );

        let err_msg = resultado.unwrap_err().to_string();
        assert!(
            err_msg.contains("FOREIGN KEY") || err_msg.contains("constraint"),
            "erro deve mencionar violação de FK, mas foi: {err_msg}"
        );
    }

    #[test]
    fn test_update_status_candidatura_valida() {
        let conn = banco_em_memoria();
        run_migrations(&conn).unwrap();

        // Inserir vaga pai
        let vaga = Vaga {
            id:       "v-001".to_string(),
            titulo:   "SRE".to_string(),
            empresa:  "Acme".to_string(),
            url:      "https://acme.com/jobs/sre".to_string(),
            descricao: None,
        };
        insert_vaga(&conn, &vaga).unwrap();

        // Inserir candidatura
        conn.execute(
            "INSERT INTO Candidatura_Forjada (id, vaga_id) VALUES ('c-001', 'v-001')",
            [],
        )
        .unwrap();

        // Transição RASCUNHO → ENVIADA
        update_status_candidatura(&conn, "c-001", CandidaturaStatus::Enviada)
            .expect("transição de status deve funcionar");

        let status: String = conn
            .query_row(
                "SELECT status FROM Candidatura_Forjada WHERE id = 'c-001'",
                [],
                |r| r.get(0),
            )
            .unwrap();
        assert_eq!(status, "ENVIADA");
    }

    #[test]
    fn test_update_status_id_inexistente_retorna_erro() {
        let conn = banco_em_memoria();
        run_migrations(&conn).unwrap();

        let resultado = update_status_candidatura(&conn, "id-fantasma", CandidaturaStatus::Enviada);
        assert!(
            resultado.is_err(),
            "deve retornar Err para ID inexistente — sem silent failures"
        );
    }
}
