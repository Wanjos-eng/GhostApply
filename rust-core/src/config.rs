//! Configuração da aplicação.
//!
//! # Intenção
//! Carrega variáveis sensíveis do `.env` *uma única vez* na inicialização.
//!
//! # Restrição (SecOps)
//! `db_encryption_key` é marcado com `Zeroize`/`ZeroizeOnDrop`: ao sair do
//! escopo (Drop), os bytes da chave são sobrescritos com zeros na RAM,
//! prevenindo ataques de extração de memória (cold-boot, /proc/mem dumps).

use anyhow::{Context, Result};
use zeroize::{Zeroize, ZeroizeOnDrop};

/// Configuração central da aplicação, carregada a partir de variáveis de ambiente.
///
/// Os campos marcados com `Zeroize` são zerados automaticamente no `Drop`.
#[derive(Debug, Zeroize, ZeroizeOnDrop)]
pub struct AppConfig {
    /// Caminho para o arquivo `.sqlite` (pode ser `:memory:` em testes).
    pub database_url: String,

    /// Chave mestra AES-256 do SQLCipher.
    ///
    /// **SEGURANÇA:** esse campo é zerado na RAM ao sair de escopo via `ZeroizeOnDrop`.
    /// Após estabelecer a conexão com o banco, descarte o `AppConfig` explicitamente
    /// para acionar o zeroize o mais cedo possível.
    pub db_encryption_key: String,

    /// Chave de API do Groq (usada para triagem).
    pub groq_api_key: String,

    /// Chave de API do Gemini (usada para geração de currículo).
    pub gemini_api_key: String,
}

impl AppConfig {
    /// Carrega a configuração a partir do `.env` e/ou variáveis de ambiente do sistema.
    ///
    /// `dotenvy::dotenv()` não falha se o arquivo `.env` não existir — apenas ignora.
    /// Isso permite que ambientes de CI/CD usem variáveis de ambiente sem `.env`.
    pub fn from_env() -> Result<Self> {
        // Carrega o .env se existir; ignora a ausência do arquivo.
        let _ = dotenvy::dotenv();

        let database_url = std::env::var("DATABASE_URL")
            .context("variável de ambiente DATABASE_URL não definida")?;

        let db_encryption_key = std::env::var("DB_ENCRYPTION_KEY")
            .context("variável de ambiente DB_ENCRYPTION_KEY não definida")?;

        let groq_api_key = std::env::var("GROQ_API_KEY")
            .context("variável de ambiente GROQ_API_KEY não definida")?;

        let gemini_api_key = std::env::var("GEMINI_API_KEY")
            .context("variável de ambiente GEMINI_API_KEY não definida")?;

        Ok(Self {
            database_url,
            db_encryption_key,
            groq_api_key,
            gemini_api_key,
        })
    }
}
