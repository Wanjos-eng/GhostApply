//! Cliente HTTP para a API do Groq (LLM rápido para triagem).
//!
//! # Intent (Tasks 26–28)
//! Classificar se uma vaga é exclusivamente remota usando um system prompt
//! rígido que força resposta SIM/NAO. Funciona como primeiro filtro do pipeline.
//!
//! # Constraint (SecOps — Task 27)
//! O reqwest é configurado com `use_rustls_tls()` para forçar TLS 1.3.
//! Nenhum payload (contendo descrições de vagas) trafega sem criptografia moderna.

use anyhow::{bail, Context, Result};
use reqwest::Client;
use serde::{Deserialize, Serialize};

/// System prompt rígido (Task 28).
/// Força a IA a retornar apenas SIM ou NAO — sem explicações, sem ambiguidade.
const SYSTEM_PROMPT: &str =
    "Analise o texto. A vaga é exclusiva para remoto? Retorne apenas a palavra SIM ou NAO";

const GROQ_API_URL: &str = "https://api.groq.com/openai/v1/chat/completions";

// ── Serde structs (Task 26) ──────────────────────────────────────────────────

#[derive(Debug, Serialize)]
pub struct GroqRequest {
    pub model: String,
    pub messages: Vec<GroqMessage>,
    pub temperature: f32,
    pub max_tokens: u32,
}

#[derive(Debug, Serialize)]
pub struct GroqMessage {
    pub role: String,
    pub content: String,
}

#[derive(Debug, Deserialize)]
pub struct GroqResponse {
    pub choices: Vec<GroqChoice>,
}

#[derive(Debug, Deserialize)]
pub struct GroqChoice {
    pub message: GroqResponseMessage,
}

#[derive(Debug, Deserialize)]
pub struct GroqResponseMessage {
    pub content: String,
}

// ── Client ───────────────────────────────────────────────────────────────────

pub struct GroqClient {
    client: Client,
    api_key: String,
    model: String,
}

impl GroqClient {
    /// Cria um novo cliente Groq com TLS 1.3 obrigatório (SecOps Task 27).
    ///
    /// `use_rustls_tls()` garante que o handshake mínimo é TLS 1.3.
    /// Se o servidor não suportar, a conexão falha — nunca degrada para TLS 1.2.
    pub fn new(api_key: String) -> Result<Self> {
        let client = Client::builder()
            .use_rustls_tls()
            .min_tls_version(reqwest::tls::Version::TLS_1_3)
            .build()
            .context("GroqClient: falha ao construir HTTP client com TLS 1.3")?;

        Ok(Self {
            client,
            api_key,
            model: "llama-3.3-70b-versatile".to_string(),
        })
    }

    /// Classifica se a vaga é exclusivamente remota.
    ///
    /// Retorna `true` se a IA responder "SIM", `false` se "NAO".
    /// Qualquer outra resposta é tratada como erro (a IA não seguiu o prompt).
    pub async fn classify_remote(&self, descricao: &str) -> Result<bool> {
        let request = GroqRequest {
            model: self.model.clone(),
            messages: vec![
                GroqMessage {
                    role: "system".to_string(),
                    content: SYSTEM_PROMPT.to_string(),
                },
                GroqMessage {
                    role: "user".to_string(),
                    content: descricao.to_string(),
                },
            ],
            temperature: 0.0, // determinístico: queremos SIM/NAO, não criatividade
            max_tokens: 5,    // SIM ou NAO = 1 token; 5 é margem de segurança
        };

        let response = self
            .client
            .post(GROQ_API_URL)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&request)
            .send()
            .await
            .context("GroqClient: falha na requisição HTTP")?;

        let body: GroqResponse = response
            .json()
            .await
            .context("GroqClient: falha ao deserializar resposta JSON")?;

        let answer = body
            .choices
            .first()
            .map(|c| c.message.content.trim().to_uppercase())
            .unwrap_or_default();

        match answer.as_str() {
            "SIM" => Ok(true),
            "NAO" | "NÃO" => Ok(false),
            other => bail!(
                "GroqClient: resposta inesperada do modelo: '{}' (esperado SIM ou NAO)",
                other
            ),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_groq_request_serialization() {
        let req = GroqRequest {
            model: "test-model".to_string(),
            messages: vec![
                GroqMessage {
                    role: "system".to_string(),
                    content: SYSTEM_PROMPT.to_string(),
                },
                GroqMessage {
                    role: "user".to_string(),
                    content: "Vaga 100% remota".to_string(),
                },
            ],
            temperature: 0.0,
            max_tokens: 5,
        };

        let json = serde_json::to_string(&req).expect("serialização deve funcionar");
        assert!(json.contains("system"));
        assert!(json.contains(SYSTEM_PROMPT));
        assert!(json.contains("test-model"));
    }
}
