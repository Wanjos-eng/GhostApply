//! Cliente HTTP para a API Google AI Studio (Gemini).
//!
//! # Intenção (Tarefas 31–32)
//! Gerar um currículo personalizado em Markdown a partir da interpolação
//! do currículo base do candidato (`meu_curriculo.md`) com a descrição da vaga.
//!
//! # Restrição
//! Toda comunicação usa TLS 1.3 (mesmo client builder do GroqClient).

use anyhow::{Context, Result};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use zeroize::Zeroize;

const GEMINI_API_URL: &str =
    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent";

const GENERATION_PROMPT: &str = r#"Você é um especialista em currículos ATS-friendly e posicionamento estratégico.

Receba dois blocos de texto:
1. CURRÍCULO BASE do candidato
2. DESCRIÇÃO DA VAGA alvo

Sua tarefa:
- Adapte o currículo para máxima relevância com a vaga, focando estritamente em IMPACTO e ARQUITETURA.
- Em vez de listar linguagens genéricas (ex: 'CRUD em Node'), force o destaque na capacidade de engenharia de software: evidencie fortemente projetos de aplicações desktop híbridas de alta performance (Wails, Go, Tauri, Rust) e construções complexas (interpretadores lógicos, AST, algoritmos avançados).
- Mantenha APENAS informações verdadeiras do currículo base.
- Use formato Markdown estruturado com seções: Resumo, Ouro Oculto (Experiência/Projetos Críticos), Habilidades, Formação.
- NÃO adicione saudações, despedidas ou comentários. Retorne APENAS o Markdown do currículo."#;

// ── Structs Serde ───────────────────────────────────────────────────────────

#[derive(Debug, Serialize)]
pub struct GeminiRequest {
    pub contents: Vec<GeminiContent>,
}

#[derive(Debug, Serialize)]
pub struct GeminiContent {
    pub parts: Vec<GeminiPart>,
}

#[derive(Debug, Serialize)]
pub struct GeminiPart {
    pub text: String,
}

#[derive(Debug, Deserialize)]
pub struct GeminiResponse {
    pub candidates: Option<Vec<GeminiCandidate>>,
}

#[derive(Debug, Deserialize)]
pub struct GeminiCandidate {
    pub content: GeminiCandidateContent,
}

#[derive(Debug, Deserialize)]
pub struct GeminiCandidateContent {
    pub parts: Vec<GeminiResponsePart>,
}

#[derive(Debug, Deserialize)]
pub struct GeminiResponsePart {
    pub text: String,
}

// ── Cliente ─────────────────────────────────────────────────────────────────

pub struct GeminiClient {
    client: Client,
    api_key: String,
}

impl GeminiClient {
    /// Cria um novo cliente Gemini com TLS 1.3 obrigatório.
    pub fn new(api_key: String) -> Result<Self> {
        let client = Client::builder()
            .use_rustls_tls()
            .min_tls_version(reqwest::tls::Version::TLS_1_3)
            .build()
            .context("GeminiClient: falha ao construir HTTP client com TLS 1.3")?;

        Ok(Self { client, api_key })
    }

    /// Gera um currículo adaptado interpolando currículo base + descrição da vaga.
    ///
    /// # Intenção
    /// O prompt força o Gemini a retornar APENAS Markdown — sem greetings nem
    /// comentários. O chamador pode usar `pdf::generator::extract_markdown_block`
    /// como segunda camada de limpeza.
    pub async fn gerar_curriculo(
        &self,
        curriculo_md: &str,
        descricao_vaga: &str,
    ) -> Result<String> {
        let user_text = format!(
            "{}\n\n--- CURRÍCULO BASE ---\n{}\n\n--- DESCRIÇÃO DA VAGA ---\n{}",
            GENERATION_PROMPT, curriculo_md, descricao_vaga
        );

        let request = GeminiRequest {
            contents: vec![GeminiContent {
                parts: vec![GeminiPart { text: user_text }],
            }],
        };

        let response = self
            .client
            .post(GEMINI_API_URL)
            .header("x-goog-api-key", &self.api_key)
            .json(&request)
            .send()
            .await
            .context("GeminiClient: falha na requisição HTTP")?;

        let status = response.status();
        if !status.is_success() {
            let body = response.text().await.unwrap_or_default();
            anyhow::bail!(
                "GeminiClient: API retornou HTTP {} — {}",
                status, body
            );
        }

        let body: GeminiResponse = response
            .json()
            .await
            .context("GeminiClient: falha ao deserializar resposta JSON")?;

        let text = body
            .candidates
            .and_then(|c| c.into_iter().next())
            .map(|c| {
                c.content
                    .parts
                    .into_iter()
                    .map(|p| p.text)
                    .collect::<Vec<_>>()
                    .join("\n")
            })
            .unwrap_or_default();

        if text.is_empty() {
            anyhow::bail!("GeminiClient: resposta vazia do modelo");
        }

        Ok(text)
    }
}

impl Drop for GeminiClient {
    fn drop(&mut self) {
        self.api_key.zeroize();
    }
}
