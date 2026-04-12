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
use zeroize::Zeroize;

/// System prompt rígido.
/// Força a IA a retornar apenas SIM, NAO ou ALERTA_MANUAL — sem explicações.
const SYSTEM_PROMPT: &str =
    "Você é um classificador rigoroso. Avalie a vaga de TI. 
Regra 1: Se for um Programa de Talentos Premium de grandes empresas (Bancos, Bolsas de Valores, Fintechs, BigTechs), retorne APENAS a palavra ALERTA_MANUAL.
Regra 2: Se não for programa de talentos, mas for exclusiva para 100% Remoto, retorne APENAS a palavra SIM.
Regra 3: Se não for remota, ou for híbrida/presencial, retorne APENAS a palavra NAO.
Atenção: Não justifique nem cumprimente. Sua saída será mapeada diretamente num banco de dados. Seja estrito.";

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

    /// Classifica o status da vaga baseado no Modo Sniper.
    /// Retorna `"SIM"`, `"NAO"`, ou `"ALERTA_MANUAL"`.
    pub async fn classify_remote(&self, descricao: &str) -> Result<String> {
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
            temperature: 0.1, // temperatura muito baixa para deter alucinações de formatação
            max_tokens: 10,   // "ALERTA_MANUAL" requer margem de tokens levemente maior
        };

        let response = self
            .client
            .post(GROQ_API_URL)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&request)
            .send()
            .await
            .context("GroqClient: falha na requisição HTTP")?;

        let status = response.status();
        if !status.is_success() {
            let body = response.text().await.unwrap_or_default();
            bail!(
                "GroqClient: API retornou HTTP {} — {}",
                status, body
            );
        }

        let body: GroqResponse = response
            .json()
            .await
            .context("GroqClient: falha ao deserializar resposta JSON")?;

        let answer = body
            .choices
            .first()
            .map(|c| c.message.content.trim().to_uppercase())
            .unwrap_or_default();

        if answer.contains("ALERTA_MANUAL") {
            return Ok("ALERTA_MANUAL".to_string());
        }
        if answer.contains("SIM") {
            return Ok("SIM".to_string());
        }
        if answer.contains("NAO") || answer.contains("NÃO") {
            return Ok("NAO".to_string());
        }

        bail!(
            "GroqClient: resposta inesperada do modelo: '{}' (esperado SIM, NAO ou ALERTA_MANUAL)",
            answer
        )
    }
}

impl Drop for GroqClient {
    fn drop(&mut self) {
        self.api_key.zeroize();
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
        assert!(json.contains("ALERTA_MANUAL")); // Testa sub-string segura contra \n JSON do Prompt Completo
        assert!(json.contains("test-model"));
    }
}
