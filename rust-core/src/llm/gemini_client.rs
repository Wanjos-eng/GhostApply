//! Cliente HTTP para a API Google AI Studio (Gemini).
//!
//! # Intenção
//! Gerar um currículo personalizado em Markdown a partir da interpolação
//! do currículo base do candidato (`meu_curriculo.md`) com a descrição da vaga.
//!
//! # Restrição
//! Toda comunicação usa TLS 1.3 (mesmo client builder do GroqClient).

use anyhow::{Context, Result};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::sync::Mutex;
use std::time::{Duration, Instant};
use zeroize::Zeroize;

const GEMINI_API_BASE_URL: &str = "https://generativelanguage.googleapis.com/v1beta";
const DEFAULT_GEMINI_MODEL: &str = "gemini-2.0-flash";
const MODEL_CACHE_TTL: Duration = Duration::from_secs(20 * 60);

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

#[derive(Debug, Deserialize)]
struct GeminiListModelsResponse {
    models: Option<Vec<GeminiModelInfo>>,
}

#[derive(Debug, Deserialize)]
struct GeminiModelInfo {
    name: String,
    #[serde(default, rename = "supportedGenerationMethods")]
    supported_generation_methods: Vec<String>,
}

#[derive(Debug, Default)]
struct GeminiModelCache {
    model: String,
    expires_at: Option<Instant>,
}

// ── Cliente ─────────────────────────────────────────────────────────────────

pub struct GeminiClient {
    client: Client,
    api_key: String,
    model_cache: Mutex<GeminiModelCache>,
}

impl GeminiClient {
    /// Cria um novo cliente Gemini com TLS 1.3 obrigatório.
    pub fn new(api_key: String) -> Result<Self> {
        let client = Client::builder()
            .use_rustls_tls()
            .min_tls_version(reqwest::tls::Version::TLS_1_3)
            .build()
            .context("GeminiClient: falha ao construir HTTP client com TLS 1.3")?;

        Ok(Self {
            client,
            api_key,
            model_cache: Mutex::new(GeminiModelCache::default()),
        })
    }

    fn generate_content_url(model: &str) -> String {
        format!(
            "{}/models/{}:generateContent",
            GEMINI_API_BASE_URL,
            model.trim()
        )
    }

    fn supports_generate_content(methods: &[String]) -> bool {
        methods
            .iter()
            .any(|m| m.trim().eq_ignore_ascii_case("generateContent"))
    }

    fn invalidate_model_cache(&self) {
        if let Ok(mut cache) = self.model_cache.lock() {
            *cache = GeminiModelCache::default();
        }
    }

    async fn fetch_generate_content_models(&self) -> Result<Vec<String>> {
        let response = self
            .client
            .get(format!("{}/models?pageSize=100", GEMINI_API_BASE_URL))
            .header("x-goog-api-key", &self.api_key)
            .send()
            .await
            .context("GeminiClient: falha ao listar modelos")?;

        let status = response.status();
        if !status.is_success() {
            let body = response.text().await.unwrap_or_default();
            anyhow::bail!(
                "GeminiClient: list models retornou HTTP {} — {}",
                status,
                body
            );
        }

        let payload: GeminiListModelsResponse = response
            .json()
            .await
            .context("GeminiClient: falha ao deserializar list models")?;

        let models = payload
            .models
            .unwrap_or_default()
            .into_iter()
            .filter(|m| Self::supports_generate_content(&m.supported_generation_methods))
            .map(|m| m.name.trim().trim_start_matches("models/").to_string())
            .filter(|m| !m.is_empty())
            .collect::<Vec<_>>();

        Ok(models)
    }

    async fn resolve_model(&self, force_refresh: bool) -> String {
        if !force_refresh {
            if let Ok(cache) = self.model_cache.lock() {
                if !cache.model.is_empty()
                    && cache
                        .expires_at
                        .map(|exp| Instant::now() < exp)
                        .unwrap_or(false)
                {
                    return cache.model.clone();
                }
            }
        }

        let selected = match self.fetch_generate_content_models().await {
            Ok(models) => models
                .into_iter()
                .next()
                .unwrap_or_else(|| DEFAULT_GEMINI_MODEL.to_string()),
            Err(_) => DEFAULT_GEMINI_MODEL.to_string(),
        };

        if let Ok(mut cache) = self.model_cache.lock() {
            cache.model = selected.clone();
            cache.expires_at = Some(Instant::now() + MODEL_CACHE_TTL);
        }

        selected
    }

    async fn send_generate_content(
        &self,
        model: &str,
        request: &GeminiRequest,
    ) -> Result<reqwest::Response> {
        self.client
            .post(Self::generate_content_url(model))
            .header("x-goog-api-key", &self.api_key)
            .json(request)
            .send()
            .await
            .context("GeminiClient: falha na requisição HTTP")
    }

    /// Gera um currículo adaptado interpolando currículo base + descrição da vaga.
    ///
    /// # Intenção
    /// O prompt força o Gemini a retornar APENAS Markdown — sem saudações nem
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

        let mut model = self.resolve_model(false).await;
        let mut response = self.send_generate_content(&model, &request).await?;
        if response.status() == reqwest::StatusCode::NOT_FOUND {
            self.invalidate_model_cache();
            model = self.resolve_model(true).await;
            response = self.send_generate_content(&model, &request).await?;
        }

        let status = response.status();
        if !status.is_success() {
            let body = response.text().await.unwrap_or_default();
            anyhow::bail!(
                "GeminiClient: API retornou HTTP {} (model={}) — {}",
                status,
                model,
                body
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
