//! Worker que orquestra o pipeline multi-agente.
//!
//! # Fluxo (Tasks 29–30, 38)
//! 1. Lê vagas `PENDENTE` do banco
//! 2. Groq classifica se é remoto → NAO = `REJEITADO_PRESENCIAL`
//! 3. Gemini gera currículo adaptado em Markdown
//! 4. PDF é gerado e salvo em `/tmp/forja/{uuid}-cv.pdf`
//! 5. Path do PDF é registrado em `Candidatura_Forjada` com status `FORJADO`

use std::path::Path;

use anyhow::{Context, Result};
use rusqlite::Connection;

use crate::llm::gemini_client::GeminiClient;
use crate::llm::groq_client::GroqClient;
use crate::pdf::generator;

/// Vaga pendente carregada do banco para processamento.
struct VagaPendente {
    id: String,
    descricao: String,
}

/// Executa o pipeline multi-agente para todas as vagas PENDENTE.
///
/// # Parâmetros
/// - `conn`: conexão SQLCipher já autenticada
/// - `groq`: cliente Groq para triagem remoto/presencial
/// - `gemini`: cliente Gemini para geração de currículo
/// - `curriculo_base`: conteúdo de `meu_curriculo.md`
/// - `output_dir`: diretório para salvar PDFs (padrão: `/tmp/forja/`)
pub async fn run_worker(
    conn: &Connection,
    groq: &GroqClient,
    gemini: &GeminiClient,
    curriculo_base: &str,
    output_dir: &Path,
) -> Result<()> {
    let vagas = listar_vagas_pendentes(conn)?;
    println!("worker: {} vagas pendentes para processar", vagas.len());

    for vaga in &vagas {
        if let Err(e) = processar_vaga(conn, groq, gemini, curriculo_base, output_dir, vaga).await
        {
            // Soft failure: log e continua com próxima vaga
            eprintln!("worker: falha ao processar vaga {}: {}", vaga.id, e);
        }
    }

    Ok(())
}

/// Processa uma única vaga pelo pipeline completo.
async fn processar_vaga(
    conn: &Connection,
    groq: &GroqClient,
    gemini: &GeminiClient,
    curriculo_base: &str,
    output_dir: &Path,
    vaga: &VagaPendente,
) -> Result<()> {
    // ── Task 29: Triagem Groq ────────────────────────────────────────────────
    let classificacao = groq
        .classify_remote(&vaga.descricao)
        .await
        .with_context(|| format!("triagem Groq falhou para vaga {}", vaga.id))?;

    // ── Modos de Triagem e Alertas ────────────────────────────────────────
    if classificacao == "ALERTA_MANUAL" {
        conn.execute(
            "UPDATE Vaga_Prospectada SET status = 'ALERTA_MANUAL' WHERE id = ?1",
            rusqlite::params![vaga.id],
        )
        .with_context(|| format!("falha ao criar alerta na vaga {}", vaga.id))?;

        println!("worker: vaga {} → ALERTA_MANUAL (Talent Program Detectado)", vaga.id);
        return Ok(());
    }

    if classificacao == "NAO" {
        conn.execute(
            "UPDATE Vaga_Prospectada SET status = 'REJEITADO_PRESENCIAL' WHERE id = ?1",
            rusqlite::params![vaga.id],
        )
        .with_context(|| format!("falha ao rejeitar vaga {}", vaga.id))?;

        println!("worker: vaga {} → REJEITADO_PRESENCIAL", vaga.id);
        return Ok(());
    }

    // ── Task 32: Gerar currículo adaptado via Gemini ─────────────────────────
    let raw_output = gemini
        .gerar_curriculo(curriculo_base, &vaga.descricao)
        .await
        .with_context(|| format!("geração Gemini falhou para vaga {}", vaga.id))?;

    // ── Task 33: Extrair bloco Markdown limpo ────────────────────────────────
    let clean_md = generator::extract_markdown_block(&raw_output);

    // ── Tasks 36–37: Gerar PDF em /tmp/forja/{uuid}-cv.pdf ───────────────────
    let pdf_path = generator::render_to_pdf(&clean_md, output_dir)
        .with_context(|| format!("geração PDF falhou para vaga {}", vaga.id))?;

    let pdf_path_str = pdf_path.to_string_lossy().to_string();

    // ── Task 38: Registrar candidatura e atualizar status ────────────────────
    let candidatura_id = uuid::Uuid::new_v4().to_string();
    conn.execute(
        "INSERT INTO Candidatura_Forjada (id, vaga_id, curriculo_path, status)
         VALUES (?1, ?2, ?3, 'FORJADO')",
        rusqlite::params![candidatura_id, vaga.id, pdf_path_str],
    )
    .with_context(|| format!("falha ao registrar candidatura para vaga {}", vaga.id))?;

    conn.execute(
        "UPDATE Vaga_Prospectada SET status = 'ANALISADA' WHERE id = ?1",
        rusqlite::params![vaga.id],
    )
    .with_context(|| format!("falha ao atualizar status da vaga {}", vaga.id))?;

    println!(
        "worker: vaga {} → FORJADO (PDF: {})",
        vaga.id,
        pdf_path_str
    );

    Ok(())
}

/// Carrega todas as vagas com status PENDENTE do banco.
fn listar_vagas_pendentes(conn: &Connection) -> Result<Vec<VagaPendente>> {
    let mut stmt = conn
        .prepare("SELECT id, descricao FROM Vaga_Prospectada WHERE status = 'PENDENTE'")
        .context("falha ao preparar query de vagas pendentes")?;

    let vagas = stmt
        .query_map([], |row| {
            Ok(VagaPendente {
                id: row.get(0)?,
                descricao: row.get::<_, Option<String>>(1)?.unwrap_or_default(),
            })
        })
        .context("falha ao consultar vagas pendentes")?
        .collect::<Result<Vec<_>, _>>()
        .context("falha ao iterar resultados")?;

    Ok(vagas)
}
