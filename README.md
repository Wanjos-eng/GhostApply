# GhostApply

Plataforma de automação para prospecção e candidatura de vagas remotas, com painel desktop (Wails), pipeline de scraping/forja em Go e núcleo complementar em Rust.

## Visão Geral

O projeto combina 3 stacks no mesmo produto:

- Go: backend principal, automação, acesso a banco, integração IMAP/LLM e comandos de execução.
- TypeScript + React: frontend desktop do dashboard, com visualização operacional em tempo real.
- Rust: núcleo de suporte para tarefas de alto desempenho (módulos de worker/DB/PDF no rust-core).

Interface desktop e ponte entre frontend/backend são feitas por Wails.

## Screenshots do App

Prints reais da interface em funcionamento:

| Dashboard | Histórico |
| --- | --- |
| ![Dashboard](IMG/print1.png) | ![Histórico](IMG/print2.png) |

| Vagas Prospectadas | Configurações |
| --- | --- |
| ![Vagas Prospectadas](IMG/print3.png) | ![Configurações](IMG/print4.png) |

## Release Oficial

Download da release atual:

- https://github.com/Wanjos-eng/GhostApply/releases/tag/v1.0.0

Arquivos disponíveis na release:

- GhostApply-linux-amd64-bundle.zip
- GhostApply-windows-amd64-bundle.zip
- GhostApply-amd64-installer.exe

## Como Usar (Instalar e Rodar)

### Recomendado: Bundles ZIP (com filler incluso)

1. Baixe um dos bundles na release:
	- GhostApply-linux-amd64-bundle.zip
	- GhostApply-windows-amd64-bundle.zip
2. Extraia o zip.
3. Execute o app mantendo o arquivo `filler`/`filler.exe` na mesma pasta do executável principal.

### Windows (instalador NSIS)

1. Baixe `GhostApply-amd64-installer.exe`.
2. Instale normalmente.
3. Abra o app pelo atalho criado no menu iniciar ou desktop.

### Linux (amd64)

1. Baixe e extraia `GhostApply-linux-amd64-bundle.zip`.
2. Torne o binário executável:

```bash
chmod +x GhostApply filler
```

3. Execute o app:

```bash
./GhostApply
```

### Windows (amd64)

1. Baixe e extraia `GhostApply-windows-amd64-bundle.zip`.
2. Dê duplo clique para executar.
3. Se o SmartScreen alertar, clique em Mais Informações e depois em Executar assim mesmo.

### Configuração Inicial Recomendada

Ao abrir o app pela primeira vez:

1. Preencha as credenciais na tela Settings (LLM e IMAP).
2. Teste a conexão IMAP no botão Test Connection.
3. Rode o scraper para popular vagas e depois o fluxo de candidatura.

### Ajuste ATS Recomendado

O pipeline de forja usa `ATS_MIN_SCORE` para decidir se o currículo gerado está aderente o bastante para ATS.

- Regra: se o score ficar abaixo do mínimo, a vaga vai para `ALERTA_MANUAL`.
- Faixa aceita: `0.00` a `1.00`.
- Também aceita percentual inteiro: `65` equivale a `0.65`.
- Valor padrão de bootstrap: `0.40`.

Recomendação inicial por senioridade:

1. Júnior: `0.35`
2. Pleno: `0.45`
3. Sênior/Staff: `0.55`

Exemplo no `.env` do app:

```env
ATS_MIN_SCORE=0.45
```

Calibração prática:

1. Se muitas vagas boas caírem em `ALERTA_MANUAL`, reduza em `0.05`.
2. Se currículos pouco aderentes estiverem passando, aumente em `0.05`.
3. Evite valores acima de `0.70` no início para não bloquear oportunidades reais.

## Arquitetura do Repositório

Estrutura de alto nível:

- cmd: executáveis Go de pipeline.
- dashboard: app desktop Wails (backend Go + frontend React).
- internal: módulos de domínio, infra, banco e parser em Go.
- rust-core: núcleo Rust para capacidades específicas.
- scripts: automações de workflow (como commits atômicos).

Principais executáveis:

- cmd/scraper: coleta vagas e popula Vaga_Prospectada.
- cmd/filler: aplica em candidaturas FORJADO.
- dashboard: interface operacional para monitorar e comandar o pipeline.

## Como Backend e Frontend Conversam

O backend expõe métodos em dashboard/app.go no struct App. Esses métodos são publicados pelo Wails e ficam acessíveis no frontend via window.go.main.App.

Fluxo da chamada:

1. React chama método (ex.: FetchHistory, StartDaemon).
2. Wails serializa argumentos e envia ao backend Go.
3. Backend executa regra/IO e retorna DTO JSON.
4. Wails desserializa no frontend e React atualiza o estado.

Bindings gerados:

- dashboard/frontend/wailsjs/go/main/App.js
- dashboard/frontend/wailsjs/go/main/App.d.ts
- dashboard/frontend/wailsjs/go/models.ts

## Multi-Linguagem em Produção

O projeto opera com responsabilidades separadas:

- Go (orquestração): controla banco SQLite, sync de emails, chamadas de IA e subprocessos.
- React/TS (apresentação): dashboard, histórico, profile, settings e relatórios.
- Rust (núcleo): módulos de performance e utilitários de geração/processamento avançado.

Esse desenho reduz acoplamento:

- UI não conhece SQL ou detalhes de IMAP/LLM.
- Backend não conhece estado visual do React.
- Rust entra como engine especializado sem vazar para o domínio de UI.

## Banco de Dados

Tabela principal de vagas e lifecycle:

- Vaga_Prospectada

Tabela de candidatura por vaga:

- Candidatura_Forjada

Tabela de e-mails classificados:

- Email_Recrutador

O dashboard inicializa schema mínimo no startup para evitar crash da UI quando DB local estiver vazio.

## Fluxos Funcionais

### 1) Vagas e candidaturas

1. Scraper grava Vaga_Prospectada.
2. Pipeline classifica e gera FORJADO.
3. BaseProfile aciona StartDaemon.
4. StartDaemon sobe cmd/filler em background.
5. Filler aplica nas vagas FORJADO e atualiza status.

### 2) Inbox e entrevistas

1. SyncEmailsRoutine conecta IMAP.
2. Cohere classifica mensagem (ENTREVISTA/OUTRO etc).
3. Email_Recrutador é persistida.
4. Dashboard e Reports consomem via FetchEmails/FetchInterviews.

### 3) Dossiê

1. Usuário seleciona entrevista no frontend.
2. Front chama GerarDossieEstudos.
3. Backend consulta Gemini e retorna texto estruturado.
4. Front renderiza e permite exportação de arquivo.

## Performance na Tela

O dashboard chama `RunPerformanceSuite` no backend Go. A coleta roda em 21 amostras e mostra p50/p95/p99 na UI.

### Resultado coletado em 12/04/2026

Ambiente local, banco vazio, sem IMAP configurado.

| Métrica | p50 | p95 | p99 |
| --- | ---: | ---: | ---: |
| DB Ping | 0.000 ms | 0.001 ms | 0.001 ms |
| Fetch History | 0.028 ms | 0.111 ms | 0.111 ms |
| Fetch Emails | 0.015 ms | 0.052 ms | 0.052 ms |
| Suite Total | 0.062 ms | 0.258 ms | 0.258 ms |

Outros números da coleta:

- samples: 21
- history_rows: 0
- email_rows: 0
- interview_rows: 0
- database_reachable: true

Na UI essa mesma coleta aparece em `dashboard/frontend/src/components/DashboardView.tsx`.

## Testes de Performance (Go + Rust)

Guia prático para medir desempenho real do sistema em ambiente local.

### O que medir (essencial)

- Latência por operação: p50, p95 e p99.
- Throughput: operações por segundo em carga estável.
- Uso de memória: pico e crescimento durante execução contínua.
- Tempo de cold start: inicialização do serviço e primeira resposta útil.
- Estabilidade sob repetição: variância entre rodadas.

### Backend Go (orquestração Wails + DB + integrações)

Focos críticos:

- Latência de acesso ao SQLite (FetchHistory / FetchEmails / FetchInterviews).
- Tempo de resposta de métodos chamados pelo frontend via Wails.
- Custo de startup do dashboard (open DB + schema + sync inicial).

Comandos sugeridos:

1. Build de sanidade:
	go build ./...
2. Benchmarks Go (quando houver arquivos *_test.go com benchmark):
	go test -bench=. -benchmem ./...
3. Repetição para reduzir ruído (ex.: 5 rodadas):
	for i in 1 2 3 4 5; do go test -bench=. -benchmem ./...; done

Métricas para reportar:

- ns/op, B/op e allocs/op dos endpoints de leitura.
- p50/p95/p99 da suíte RunPerformanceSuite na UI.
- p95 dos métodos chamados no Dashboard em janela de 1 minuto.

### Backend Rust (rust-core: worker/DB/PDF)

Focos críticos:

- Tempo de geração de artefatos (ex.: PDF).
- Throughput do worker assíncrono.
- Overhead de serialização e persistência.

Comandos sugeridos:

1. Testes em release (baseline realista):
	cd rust-core && cargo test --release
2. Medição de tempo por teste:
	cd rust-core && cargo test --release -- --nocapture
3. Bench dedicado (quando houver pasta benches):
	cd rust-core && cargo bench

Métricas para reportar:

- Tempo médio e p95 de geração de PDF.
- Throughput do worker (jobs/minuto).
- Tempo de migração e query crítica em banco criptografado.

### Resultado coletado no Rust

`cargo test --release -- --nocapture`

- release build concluído em 39.57 s
- 9 testes passaram
- 0 falhas

Esse número é de suíte em release, não de benchmark dedicado. Ainda não tem `cargo bench` no repositório.

### Cenários recomendados de validação

- Cenário A: banco vazio (cold start).
- Cenário B: banco com volume realista (ex.: 1k vagas, 5k emails).
- Cenário C: burst de leituras simultâneas no dashboard.
- Cenário D: geração de dossiês em sequência (N execuções).

Formato recomendado de registro dos resultados:

- Ambiente de teste (CPU, RAM, SO, versão Go/Rust).
- Comando exato executado.
- Tabela com p50/p95/p99, throughput e memória.
- Comparativo antes/depois para cada otimização.

### Critérios de aceitação internos

- Dashboard não deve travar durante coleta de métricas.
- RunPerformanceSuite deve finalizar de forma consistente e previsível.
- Regressão > 20% em p95 exige investigação antes de merge.
- Toda otimização deve vir com medição reproduzível.

## Setup Local

Pré-requisitos:

- Go instalado
- Node.js e npm instalados
- Wails CLI instalado

Dependências Go:

1. go mod tidy

Frontend dashboard:

1. cd dashboard/frontend
2. npm ci

## Executar

Na raiz do dashboard:

1. cd dashboard
2. wails dev

Executáveis batch:

- go run ./cmd/scraper
- go run ./cmd/filler

## Build

Backend e executáveis Go:

1. go build ./...

Frontend:

1. cd dashboard/frontend
2. npm run build

## Testes e Verificação

Checks recomendados no fluxo local:

1. go build ./...
2. cd dashboard/frontend && npm run build
3. Validar dashboard em runtime Wails

Validação funcional mínima:

- Sidebar mostra status real de conexões.
- Dashboard mostra dados reais (sem placeholders estáticos).
- Reports lista entrevistas reais e gera dossiê.
- BaseProfile aciona StartDaemon com retorno de sucesso.

## Segurança e Operação

Observações atuais:

- Settings salva credenciais em .env local.
- Não versionar .env.
- Em ambiente real, migrar segredos para cofre/secret manager.

## Convenções de Commit

Script de commits atômicos:

- scripts/atomic_commit.sh

Padrão adotado:

- feat(scope): nova funcionalidade
- fix(scope): correção
- refactor(scope): melhoria interna
- docs(scope): documentação
- chore(scope): manutenção/tooling
