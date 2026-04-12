# Dashboard (Wails)

Aplicação desktop operacional do GhostApply.

## Stack

- Backend desktop: Go (arquivo principal em dashboard/app.go)
- Frontend desktop: React + TypeScript (dashboard/frontend/src)
- Bridge: Wails runtime + bindings gerados em dashboard/frontend/wailsjs

## Responsabilidades

Esta camada concentra o controle em tempo real do pipeline:

- Carregar/salvar configurações (Settings)
- Validar IMAP
- Ler histórico e emails classificados
- Acionar StartDaemon para execução do filler
- Gerar dossiê via Gemini
- Exibir status de saúde e métricas de performance

## Métodos Backend Expostos para o Front

Exemplos de métodos do App consumidos pela UI:

- FetchEmails
- FetchHistory
- FetchInterviews
- GetSystemStatus
- RunPerformanceSuite
- UploadAndParseCV
- StartDaemon
- LoadSettings / SaveSettings / VerifyIMAP

Os bindings do Wails são gerados em:

- frontend/wailsjs/go/main/App.js
- frontend/wailsjs/go/main/App.d.ts
- frontend/wailsjs/go/models.ts

## Como Rodar em Desenvolvimento

1. cd dashboard/frontend && npm ci
2. cd ..
3. wails dev

## Como Buildar

1. cd dashboard/frontend && npm run build
2. cd ..
3. wails build

## Performance no Dashboard

O dashboard inclui uma suíte de medições de backend:

- DB Ping (ms)
- Fetch History (ms)
- Fetch Emails (ms)
- Tempo total da suíte (ms)

Ela roda automaticamente e também pode ser disparada por botão na tela.
