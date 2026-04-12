//! Worker que executa a fila de vagas pendentes.
//!
//! Fluxo do worker:
//! 1. Busca vagas com status `PENDENTE`
//! 2. Usa Groq para separar remoto de presencial
//! 3. Usa Gemini para montar o currículo adaptado
//! 4. Gera o PDF em disco
//! 5. Registra a candidatura com status `FORJADO`

use std::path::Path;

use anyhow::{Context, Result};
use rusqlite::Connection;

use crate::llm::gemini_client::GeminiClient;
use crate::llm::groq_client::GroqClient;
use crate::pdf::generator;

/// Estrutura interna para uma vaga pendente carregada do banco.
struct VagaPendente {
    id: String,
    descricao: String,
}

/// Processa todas as vagas pendentes em sequência.
///
/// `conn` precisa estar aberto e autenticado.
/// `groq` faz a triagem inicial.
/// `gemini` gera o currículo adaptado.
/// `curriculo_base` é o texto base usado como entrada.
/// `output_dir` define onde os PDFs vão ser salvos.
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
            // Falha isolada não para a fila inteira.
            eprintln!("worker: falha ao processar vaga {}: {}", vaga.id, e);
        }
    }

    Ok(())
}

/// Processa uma vaga do começo ao fim.
async fn processar_vaga(
    conn: &Connection,
    groq: &GroqClient,
    gemini: &GeminiClient,
    curriculo_base: &str,
    output_dir: &Path,
    vaga: &VagaPendente,
) -> Result<()> {
    // Triagem inicial com Groq.
    let classificacao = groq
        .classify_remote(&vaga.descricao)
        .await
        .with_context(|| format!("triagem Groq falhou para vaga {}", vaga.id))?;

    // Trate primeiro os caminhos especiais: alerta manual ou descarte.
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

    // Gera o currículo adaptado com Gemini.
    let raw_output = gemini
        .gerar_curriculo(curriculo_base, &vaga.descricao)
        .await
        .with_context(|| format!("geração Gemini falhou para vaga {}", vaga.id))?;

    // Mantém apenas o bloco Markdown útil para o PDF.
    let clean_md = generator::extract_markdown_block(&raw_output);

    // Renderiza o PDF final no diretório de saída.
    let pdf_path = generator::render_to_pdf(&clean_md, output_dir)
        .with_context(|| format!("geração PDF falhou para vaga {}", vaga.id))?;

    let pdf_path_str = pdf_path.to_string_lossy().to_string();

    // Registra a candidatura e atualiza o status da vaga.
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

/// Busca todas as vagas pendentes no banco.
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
