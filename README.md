<!-- PROJECT_METADATA
{
  "title": "GhostApply",
  "short_description": "Plataforma desktop de automação inteligente para prospecção e candidatura em vagas remotas, construída com Go, React, Wails e Rust.",
  "primary_stack": ["Go", "React", "TypeScript", "Wails", "Rust", "SQLite"],
  "architecture": "Desktop App",
  "detail_description": "GhostApply é uma aplicação desktop multiplataforma (Linux/Windows) que automatiza o ciclo completo de busca e candidatura em vagas remotas. O backend em Go orquestra um pipeline de scraping com módulos dedicados, um núcleo complementar em Rust para processamento de alta performance, e persistência local com SQLite. O frontend em React (via Wails) entrega um painel em tempo real com rastreamento do pipeline de vagas. Projetado para ser distribuído como binário standalone sem dependências externas.",
  "images": ["IMG/print1.png", "IMG/print2.png", "IMG/print3.png", "IMG/print4.png"],
  "cover_image": "IMG/print1.png",
  "release_url": "https://github.com/Wanjos-eng/GhostApply/releases/tag/v1.0.0"
}
-->

# GhostApply

Plataforma de automação para prospecção e candidatura de vagas remotas, com painel desktop (Wails), pipeline de scraping/forja em Go e núcleo complementar em Rust.

## Visão Geral

GhostApply automatiza o ciclo completo de busca de emprego remoto:
- **Scraping inteligente** de vagas em múltiplas plataformas
- **Geração de candidaturas personalizadas** com IA integrada
- **Dashboard em tempo real** para rastrear o pipeline completo de vagas
- **Distribuição como binário standalone** — sem dependências externas

## Stack Técnica

| Camada | Tecnologia |
|--------|-----------|
| Backend / Orquestração | Go (Gin, módulos customizados) |
| Processamento Pesado | Rust (rust-core) |
| Frontend Desktop | React + TypeScript (via Wails) |
| Persistência | SQLite |
| Distribuição | Wails v2 (Linux + Windows) |

## Arquitetura

```
GhostApply/
├── cmd/             # Entrypoints da aplicação Go
├── internal/        # Lógica de negócio (pipeline, scraper, forja)
├── scraper/         # Módulos de scraping por plataforma
├── rust-core/       # Núcleo Rust para processamento de alta performance
├── dashboard/       # Frontend React (Wails bindings)
└── scripts/         # Scripts de build e deploy
```

## Como Executar

### Pré-requisitos
- Go 1.21+
- Rust + Cargo
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Node.js 18+

### Desenvolvimento
```bash
wails dev
```

### Build para produção
```bash
wails build
```

O binário gerado em `build/bin/` é autossuficiente e não requer instalação.

## Download

Baixe o binário mais recente na [página de releases](https://github.com/Wanjos-eng/GhostApply/releases/tag/v1.0.0).

## Screenshots

![Dashboard principal](./IMG/print1.png)
![Pipeline de vagas](./IMG/print2.png)
![Geração de candidatura](./IMG/print3.png)
![Configurações](./IMG/print4.png)