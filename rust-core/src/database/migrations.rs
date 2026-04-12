//! Migrations do schema do banco de dados.
//!
//! # Intent
//! Criar o schema idempotente (`IF NOT EXISTS`) em uma única transação atômica.
//! Falhar em qualquer DDL reverte tudo — nunca deixa o banco em estado parcial.
//!
//! # Tabelas
//! - `Vaga_Prospectada`   — vagas capturadas de portais (Task 07)
//! - `Candidatura_Forjada`— candidaturas geradas, FK para Vaga (Task 08)
//! - `Email_Recrutador`   — e-mails de recrutadores capturados (Task 09)

use anyhow::{Context, Result};
use rusqlite::Connection;

/// Executa todas as migrations DDL em uma transação atômica.
///
/// Idempotente: pode ser chamado ao iniciar a aplicação em toda execução.
pub fn run_migrations(conn: &Connection) -> Result<()> {
    conn.execute_batch(SCHEMA_SQL)
        .context("falha ao executar migrations — schema inválido ou banco corrompido")?;
    Ok(())
}

/// DDL completo do schema GhostApply.
///
/// Executado em bloco único dentro de `BEGIN`/`COMMIT` implícito do `execute_batch`.
const SCHEMA_SQL: &str = "
-- ── Task 07: Vaga_Prospectada ────────────────────────────────────────────────
-- Registro de vagas identificadas nos portais (fonte de verdade do pipeline).
-- `url` é UNIQUE para garantir idempotência no scraping (INSERT OR IGNORE).
-- `status` controla o ciclo de vida: NOVA → ANALISADA → DESCARTADA
CREATE TABLE IF NOT EXISTS Vaga_Prospectada (
    id        TEXT PRIMARY KEY NOT NULL,
    titulo    TEXT NOT NULL,
    empresa   TEXT NOT NULL,
    url       TEXT NOT NULL UNIQUE,
    descricao TEXT,
    status    TEXT NOT NULL DEFAULT 'NOVA'
                  CHECK (status IN ('NOVA', 'ANALISADA', 'DESCARTADA')),
    criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Task 08: Candidatura_Forjada ─────────────────────────────────────────────
-- Candidatura gerada para uma vaga (1 vaga pode ter N tentativas de envio).
-- FK com ON DELETE CASCADE: remover a vaga apaga suas candidaturas.
-- `status` segue a máquina de estados: RASCUNHO → ENVIADA → CONFIRMADA | REJEITADA
CREATE TABLE IF NOT EXISTS Candidatura_Forjada (
    id             TEXT PRIMARY KEY NOT NULL,
    vaga_id        TEXT NOT NULL
                       REFERENCES Vaga_Prospectada(id) ON DELETE CASCADE,
    curriculo_path TEXT,
    carta_path     TEXT,
    status         TEXT NOT NULL DEFAULT 'RASCUNHO'
                       CHECK (status IN ('RASCUNHO', 'ENVIADA', 'CONFIRMADA', 'REJEITADA')),
    enviado_em     TEXT,
    criado_em      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- ── Task 09: Email_Recrutador ────────────────────────────────────────────────
-- E-mails de recrutadores capturados durante o scraping ou contato.
-- `vaga_id` é nullable (ON DELETE SET NULL): e-mail persiste mesmo se a vaga for removida.
CREATE TABLE IF NOT EXISTS Email_Recrutador (
    id           TEXT PRIMARY KEY NOT NULL,
    vaga_id      TEXT REFERENCES Vaga_Prospectada(id) ON DELETE SET NULL,
    email        TEXT NOT NULL,
    nome         TEXT,
    capturado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
";
