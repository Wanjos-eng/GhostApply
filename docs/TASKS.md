# GhostApply - Backlog de Desenvolvimento

## 🎯 Objetivo: Automação Inteligente de Candidaturas (AutoApply OS)

### 📋 Tarefas em Aberto

- [ ] **Task 24: Criar rotina de limpeza de descrição de vagas.**
  - **Contexto:** O HTML da vaga vem sujo do LinkedIn.
  - **Requisitos Sênior:**
    1. Recebe uma string bruta de HTML.
    2. Retorna apenas texto puro.
    3. **SEGURANÇA:** Deve remover agressivamente tags `<script>`, e-mails e links usando Regex (Prevenção de injeção de prompt).
    4. Retorne erro se o texto final ficar vazio.
  - **Stack Sugerida:** Go (para processamento rápido de strings) ou Rust (para máxima segurança).

---

### 🏛️ Estrutura de Camadas (Clean Arch)
- `/internal/domain`: Entidades e regras de negócio puras.
- `/internal/usecase`: Lógica de orquestração.
- `/internal/infra`: Implementações concretas (Playwright, Scrapers, DB).
- `/rust-core`: Módulos de performance crítica (Rust).
