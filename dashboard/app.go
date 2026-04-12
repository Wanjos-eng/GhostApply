package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/joho/godotenv"
	"github.com/ledongthuc/pdf"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type EmailRecrutador struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Nome          string `json:"nome"`
	Classificacao string `json:"classificacao"`
	Corpo         string `json:"corpo"`
}

type VagaHistoricoDTO struct {
	VagaID            string `json:"vaga_id"`
	Titulo            string `json:"titulo"`
	Empresa           string `json:"empresa"`
	URL               string `json:"url"`
	VagaStatus        string `json:"vaga_status"`
	CandidaturaID     string `json:"candidatura_id"`
	CandidaturaStatus string `json:"candidatura_status"`
	RecrutadorNome    string `json:"recrutador_nome"`
	RecrutadorPerfil  string `json:"recrutador_perfil"`
	CriadoEm          string `json:"criado_em"`
}

type PerformanceSuiteDTO struct {
	RanAt             string `json:"ran_at"`
	Samples           int    `json:"samples"`
	DatabasePingP95MS float64 `json:"database_ping_p95_ms"`
	DatabasePingP99MS float64 `json:"database_ping_p99_ms"`
	DatabasePingMS    float64 `json:"database_ping_ms"`
	FetchHistoryP95MS float64 `json:"fetch_history_p95_ms"`
	FetchHistoryP99MS float64 `json:"fetch_history_p99_ms"`
	FetchHistoryMS    float64 `json:"fetch_history_ms"`
	FetchEmailsP95MS  float64 `json:"fetch_emails_p95_ms"`
	FetchEmailsP99MS  float64 `json:"fetch_emails_p99_ms"`
	FetchEmailsMS     float64 `json:"fetch_emails_ms"`
	FetchInterP95MS   float64 `json:"fetch_interviews_p95_ms"`
	FetchInterP99MS   float64 `json:"fetch_interviews_p99_ms"`
	FetchInterviewsMS float64 `json:"fetch_interviews_ms"`
	TotalSuiteP95MS   float64 `json:"total_suite_p95_ms"`
	TotalSuiteP99MS   float64 `json:"total_suite_p99_ms"`
	HistoryRows       int    `json:"history_rows"`
	EmailRows         int    `json:"email_rows"`
	InterviewRows     int    `json:"interview_rows"`
	TotalSuiteMS      float64 `json:"total_suite_ms"`
	DatabaseReachable bool   `json:"database_reachable"`
}

func percentileFloat64(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * percentile)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Estrutura principal da aplicação Wails.
type App struct {
	ctx      context.Context
	database *sql.DB
}

// Cria a instância principal do app.
func NewApp() *App {
	return &App{}
}

// Executa a inicialização quando o app sobe.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	godotenv.Load("../.env")

	dbPath := os.Getenv("DATABASE_URL")
	dbKey := os.Getenv("DB_ENCRYPTION_KEY")

	if dbPath == "" {
		dbPath = "../forja_ghost.sqlite"
	}

	dsn := fmt.Sprintf(
		`%s?_pragma=key('%s')&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)`,
		dbPath, dbKey,
	)

	database, err := sql.Open("sqlite3", dsn)
	if err == nil {
		err = database.Ping()
	}

	if err != nil {
		log.Printf("WAILS: Failed to open db connection: %v\n", err)
	} else {
		// Garante as tabelas mínimas para a UI não quebrar no primeiro acesso.
		_, initErr := database.Exec(`
			CREATE TABLE IF NOT EXISTS Vaga_Prospectada (
				id TEXT PRIMARY KEY NOT NULL,
				titulo TEXT NOT NULL,
				empresa TEXT NOT NULL,
				url TEXT NOT NULL UNIQUE,
				descricao TEXT,
				status TEXT NOT NULL DEFAULT 'NOVA'
					CHECK (status IN (
						'NOVA', 'PENDENTE', 'ANALISADA', 'ALERTA_MANUAL',
						'REJEITADO_PRESENCIAL', 'FORJADO', 'DESCARTADA'
					)),
				recrutador_nome TEXT,
				recrutador_perfil TEXT,
				criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			);
			CREATE TABLE IF NOT EXISTS Candidatura_Forjada (
				id TEXT PRIMARY KEY NOT NULL,
				vaga_id TEXT NOT NULL
					REFERENCES Vaga_Prospectada(id) ON DELETE CASCADE,
				curriculo_path TEXT,
				carta_path TEXT,
				status TEXT NOT NULL DEFAULT 'RASCUNHO'
					CHECK (status IN (
						'RASCUNHO', 'FORJADO', 'ENVIADA',
						'APLICADA', 'CONFIRMADA', 'REJEITADA', 'ERRO'
					)),
				enviado_em TEXT,
				criado_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			);
			CREATE TABLE IF NOT EXISTS Email_Recrutador (
				id TEXT PRIMARY KEY,
				email TEXT,
				nome TEXT,
				classificacao TEXT,
				corpo TEXT
			)
		`)
		if initErr != nil {
			log.Printf("WAILS: Failed to init DB schema: %v\n", initErr)
		}
		a.database = database
	}

	// Dispara a sincronização de emails em background para não travar a abertura.
	go a.SyncEmailsRoutine()
}

// Retorna os emails já classificados para o quadro do dashboard.
func (a *App) FetchEmails() []EmailRecrutador {
	if a.database == nil {
		return nil
	}

	rows, err := a.database.Query("SELECT id, email, COALESCE(nome, ''), classificacao, COALESCE(corpo, '') FROM Email_Recrutador")
	if err != nil {
		log.Printf("FetchEmails: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var results []EmailRecrutador
	for rows.Next() {
		var e EmailRecrutador
		if err := rows.Scan(&e.ID, &e.Email, &e.Nome, &e.Classificacao, &e.Corpo); err == nil {
			results = append(results, e)
		}
	}
	return results
}

// Filtra apenas os emails marcados como entrevista.
func (a *App) FetchInterviews() []EmailRecrutador {
	var ints []EmailRecrutador
	for _, em := range a.FetchEmails() {
		if em.Classificacao == "ENTREVISTA" {
			ints = append(ints, em)
		}
	}
	return ints
}

// Monta a visão de histórico com vagas e candidaturas.
func (a *App) FetchHistory() []VagaHistoricoDTO {
	if a.database == nil {
		return nil
	}

	query := `
		SELECT 
			v.id, v.titulo, v.empresa, v.url, v.status,
			COALESCE(v.recrutador_nome, ''), COALESCE(v.recrutador_perfil, ''),
			v.criado_em,
			COALESCE(c.id, ''), COALESCE(c.status, '')
		FROM Vaga_Prospectada v
		LEFT JOIN Candidatura_Forjada c ON v.id = c.vaga_id
		ORDER BY v.criado_em DESC
		LIMIT 1000
	`

	rows, err := a.database.Query(query)
	if err != nil {
		log.Printf("FetchHistory: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var results []VagaHistoricoDTO
	for rows.Next() {
		var h VagaHistoricoDTO
		if err := rows.Scan(
			&h.VagaID, &h.Titulo, &h.Empresa, &h.URL, &h.VagaStatus,
			&h.RecrutadorNome, &h.RecrutadorPerfil, &h.CriadoEm,
			&h.CandidaturaID, &h.CandidaturaStatus,
		); err == nil {
			results = append(results, h)
		} else {
			log.Printf("FetchHistory scan err: %v", err)
		}
	}
	return results
}

// Sincroniza a caixa de entrada e grava o resultado no banco.
func (a *App) SyncEmailsRoutine() {
	if a.database == nil {
		return
	}

	imapClient, err := NewIMAPListener()
	if err != nil {
		log.Printf("SyncEmails: Failed IMAP: %v\n", err)
		return
	}

	cohere := NewCohereClient()

	unseenMap, err := imapClient.FetchUnseenEmailBodies()
	if err != nil {
		log.Printf("SyncEmails: Failed parsing unseen: %v\n", err)
		return
	}

	for seqId, body := range unseenMap {
		classificacao, err := cohere.ClassifyEmail(body)
		if err != nil {
			classificacao = "OUTRO"
			log.Printf("SyncEmails: Cohere failure: %v\n", err)
		}

		// Salva a mensagem classificada para a UI conseguir mostrar depois.
		pseudoUUID := fmt.Sprintf("email-%d", seqId)
		_, execErr := a.database.Exec("INSERT INTO Email_Recrutador (id, email, classificacao, corpo) VALUES (?, ?, ?, ?)",
			pseudoUUID, "recruiter@example.com", classificacao, body)

		if execErr == nil {
			imapClient.MarkAsSeen(seqId)
			log.Printf("SyncEmails: Appended message [%s]", classificacao)
		}
	}
}

// Gera uma mensagem de abordagem com Cohere.
func (a *App) GenerateOutreachMessage(recruiterName, roleName string) string {
	cohere := NewCohereClient()
	msg, err := cohere.GenerateOutreachMessage(recruiterName, roleName)
	if err != nil {
		return fmt.Sprintf("Erro ao gerar Outreach: %v", err)
	}
	return msg
}

// Envia o email selecionado para o Gemini e retorna o texto do dossiê.
func (a *App) GerarDossieEstudos(emailBody string) string {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "GEMINI_API_KEY não está configurada."
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey

	prompt := `Você é um Staff Engineer e Coach Estratégico de Carreiras.
Leia a mensagem de entrevista abaixo e cruze os requisitos da vaga com a minha experiência em Arquitetura (Projetos em Wails, Go, Tauri, Rust, AST).
Diga-me EXATAMENTE:
1. Qual projeto do meu portfólio devo usar para provar qual habilidade.
2. Como funciona a cultura e as etapas de entrevista técnica para a empresa emissora.
3. Quais perguntas sistêmicas (ou de Leetcode) mais costumam cair nessa empresa para esse nível.
*OBRIGATÓRIO*: Vá à Web investigar o nome da empresa na mensagem (use suas ferramentas de busca) para me trazer dicas validadas de processos seletivos deles. Responda em Markdown estruturado.

Conteúdo do Email de Entrevista:
` + emailBody

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"tools": []map[string]interface{}{
			{
				"googleSearch": map[string]interface{}{},
			},
		},
	}

	jsonValue, _ := json.Marshal(requestBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return fmt.Sprintf("Falha ao comunicar com Gemini: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("Erro da API Gemini: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Erro ao parsear resposta: %v", err)
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text
	}

	return "Falha inesperada ao gerar dossiê."
}

// Verifica a saúde das integrações usadas pelo dashboard.
func (a *App) GetSystemStatus() map[string]interface{} {
	status := map[string]interface{}{
		"database": "✓ OK",
		"cohere":   "✗ OFFLINE",
		"groq":     "✗ OFFLINE",
		"gemini":   "✗ OFFLINE",
		"imap":     "✗ OFFLINE",
	}

	// Banco local
	if a.database != nil {
		if err := a.database.Ping(); err != nil {
			status["database"] = "✗ ERRO"
		}
	} else {
		status["database"] = "✗ ERRO"
	}

	// Cohere
	cohere := NewCohereClient()
	if cohere.apiKey != "" {
		// Faz um teste rápido na API para validar conectividade.
		resp, err := http.Post(
			"https://api.cohere.ai/v1/health",
			"application/json",
			bytes.NewBufferString(`{}`),
		)
		if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 404) {
			status["cohere"] = "✓ OK"
			resp.Body.Close()
		} else {
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// Groq
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		req, err := http.NewRequest("GET", "https://api.groq.com/", nil)
		if err == nil {
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				status["groq"] = "✓ OK"
				resp.Body.Close()
			}
		}
	}

	// Gemini
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey != "" {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent?key=%s", geminiKey)
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(`{"contents":[]}`))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				status["gemini"] = "✓ OK"
				resp.Body.Close()
			}
		}
	}

	// IMAP
	imapServer := os.Getenv("IMAP_SERVER")
	if imapServer != "" {
		_, err := NewIMAPListener()
		if err == nil {
			status["imap"] = "✓ OK"
		}
	}

	return status
}

// Executa as medições de tempo que alimentam os cards de performance do dashboard.
func (a *App) RunPerformanceSuite() PerformanceSuiteDTO {
	suiteStart := time.Now()
	result := PerformanceSuiteDTO{
		RanAt:             time.Now().UTC().Format(time.RFC3339),
		Samples:           21,
		DatabaseReachable: false,
	}

	if a.database == nil {
		result.TotalSuiteMS = float64(time.Since(suiteStart).Microseconds()) / 1000
		return result
	}

	pingRuns := make([]float64, 0, result.Samples)
	historyRuns := make([]float64, 0, result.Samples)
	emailRuns := make([]float64, 0, result.Samples)
	interviewRuns := make([]float64, 0, result.Samples)
	totalRuns := make([]float64, 0, result.Samples)

	for i := 0; i < result.Samples; i++ {
		iterStart := time.Now()

		pingStart := time.Now()
		if err := a.database.Ping(); err == nil {
			result.DatabaseReachable = true
		}
		pingRuns = append(pingRuns, float64(time.Since(pingStart).Microseconds())/1000)

		historyStart := time.Now()
		history := a.FetchHistory()
		historyRuns = append(historyRuns, float64(time.Since(historyStart).Microseconds())/1000)
		result.HistoryRows = len(history)

		emailsStart := time.Now()
		emails := a.FetchEmails()
		emailRuns = append(emailRuns, float64(time.Since(emailsStart).Microseconds())/1000)
		result.EmailRows = len(emails)

		interviewsStart := time.Now()
		interviews := a.FetchInterviews()
		interviewRuns = append(interviewRuns, float64(time.Since(interviewsStart).Microseconds())/1000)
		result.InterviewRows = len(interviews)

		totalRuns = append(totalRuns, float64(time.Since(iterStart).Microseconds())/1000)
	}

	result.DatabasePingMS = percentileFloat64(pingRuns, 0.50)
	result.DatabasePingP95MS = percentileFloat64(pingRuns, 0.95)
	result.DatabasePingP99MS = percentileFloat64(pingRuns, 0.99)

	result.FetchHistoryMS = percentileFloat64(historyRuns, 0.50)
	result.FetchHistoryP95MS = percentileFloat64(historyRuns, 0.95)
	result.FetchHistoryP99MS = percentileFloat64(historyRuns, 0.99)

	result.FetchEmailsMS = percentileFloat64(emailRuns, 0.50)
	result.FetchEmailsP95MS = percentileFloat64(emailRuns, 0.95)
	result.FetchEmailsP99MS = percentileFloat64(emailRuns, 0.99)

	result.FetchInterviewsMS = percentileFloat64(interviewRuns, 0.50)
	result.FetchInterP95MS = percentileFloat64(interviewRuns, 0.95)
	result.FetchInterP99MS = percentileFloat64(interviewRuns, 0.99)

	result.TotalSuiteMS = percentileFloat64(totalRuns, 0.50)
	result.TotalSuiteP95MS = percentileFloat64(totalRuns, 0.95)
	result.TotalSuiteP99MS = percentileFloat64(totalRuns, 0.99)
	_ = suiteStart
	return result
}

// ----------------------------------------------------
// Estruturas de settings e configuração expostas para a UI
// ----------------------------------------------------

type SettingsDTO struct {
	CohereAPIKey string `json:"cohere_api_key"`
	GroqAPIKey   string `json:"groq_api_key"`
	GeminiAPIKey string `json:"gemini_api_key"`
	ImapServer   string `json:"imap_server"`
	ImapUser     string `json:"imap_user"`
	ImapPass     string `json:"imap_pass"`
}

type ProfileDTO struct {
	TargetRoles    []string `json:"target_roles"`
	CoreStack      []string `json:"core_stack"`
	StrictlyRemote bool     `json:"strictly_remote"`
	MinSalaryFloor string   `json:"min_salary_floor"`
	AppsPerDay     int      `json:"apps_per_day"`
}

// Carrega o mapeamento local do .env para a tela de configurações do frontend.
func (a *App) LoadSettings() SettingsDTO {
	godotenv.Load("../.env")
	return SettingsDTO{
		CohereAPIKey: os.Getenv("COHERE_API_KEY"),
		GroqAPIKey:   os.Getenv("GROQ_API_KEY"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		ImapServer:   os.Getenv("IMAP_SERVER"),
		ImapUser:     os.Getenv("IMAP_USER"),
		ImapPass:     os.Getenv("IMAP_PASS"),
	}
}

// Persiste os valores do mapa de volta no .env local de forma segura.
func (a *App) SaveSettings(cfg SettingsDTO) bool {
	envMap := map[string]string{
		"COHERE_API_KEY": cfg.CohereAPIKey,
		"GROQ_API_KEY":   cfg.GroqAPIKey,
		"GEMINI_API_KEY": cfg.GeminiAPIKey,
		"IMAP_SERVER":    cfg.ImapServer,
		"IMAP_USER":      cfg.ImapUser,
		"IMAP_PASS":      cfg.ImapPass,
	}

	// Persiste no .env local.
	err := godotenv.Write(envMap, "../.env")
	if err != nil {
		log.Printf("SaveSettings: Failed to write .env: %v", err)
		return false
	}
	return true
}

// Abre o seletor nativo, extrai o texto do PDF e pede ao Gemini o JSON estruturado.
func (a *App) UploadAndParseCV() ProfileDTO {
	// 1. Abre o diálogo do sistema operacional.
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select your CV (PDF)",
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF Files (*.pdf)", Pattern: "*.pdf"},
		},
	})
	if err != nil || filePath == "" {
		log.Println("UploadAndParseCV: No file selected or error:", err)
		return ProfileDTO{}
	}

	log.Println("Parsing PDF File:", filePath)

	// 2. Lê o texto do PDF.
	f, r, err := pdf.Open(filePath)
	if err != nil {
		log.Printf("UploadAndParseCV: Fail to open PDF: %v\n", err)
		return ProfileDTO{}
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		log.Printf("UploadAndParseCV: Fail to extract text: %v\n", err)
		return ProfileDTO{}
	}
	buf.ReadFrom(b)
	textContent := buf.String()

	// 3. Monta a requisição para o Gemini estruturar TargetRoles e CoreStack.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Println("UploadAndParseCV: No GEMINI_API_KEY found")
		return ProfileDTO{}
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey

	prompt := `I am providing a CV in raw text below.
Please extract the 'TargetRoles' (which titles the candidate fits best) and 'CoreStack' (main programming languages and technologies, max 8 items).
Return ONLY a valid JSON object starting with '{' and ending with '}' with keys: "target_roles": ["..."], "core_stack": ["..."]. Do not use markdown format blocks like ` + "```json" + `.
CV Text:
` + textContent

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	jsonValue, _ := json.Marshal(requestBody)
	resp, httpErr := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
	if httpErr != nil {
		log.Printf("UploadAndParseCV: Gemini HTTP Call Failed: %v\n", httpErr)
		return ProfileDTO{}
	}
	defer resp.Body.Close()

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
		log.Printf("UploadAndParseCV: Failed to decode Gemini Response: %v", decErr)
		return ProfileDTO{}
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		geminiJSON := result.Candidates[0].Content.Parts[0].Text
		geminiJSON = string(bytes.TrimSpace([]byte(geminiJSON)))

		log.Println("Gemini Extracted JSON:", geminiJSON)
		var parsed ProfileDTO
		// Preenche os valores padrão.
		parsed.StrictlyRemote = true
		parsed.MinSalaryFloor = "$120,000"
		parsed.AppsPerDay = 50

		if err := json.Unmarshal([]byte(geminiJSON), &parsed); err != nil {
			log.Printf("UploadAndParseCV: Could not unmarshal string format: %v", err)
			return ProfileDTO{}
		}

		return parsed
	}

	return ProfileDTO{}
}

// StartDaemon inicia o job batch do filler em segundo plano.
func (a *App) StartDaemon(cfg ProfileDTO) bool {
	log.Printf("🚀 WAILS: Launching Filler Daemon with config: %+v", cfg)

	// Sobe o filler como subprocesso em background sem travar a UI.
	go func() {
		// Usa go run a partir da raiz do repositório durante o desenvolvimento.
		cmd := exec.Command("go", "run", "./cmd/filler")

		// Executa a partir da raiz do projeto para o filler resolver os caminhos.
		cmd.Dir = ".."

		// Herda o ambiente atual; o filler lê o .env na inicialização.
		cmd.Env = os.Environ()

		// Captura stdout e stderr para diagnóstico se o processo encerrar cedo.
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Executa de forma síncrona dentro da goroutine.
		if err := cmd.Run(); err != nil {
			log.Printf("❌ Filler exited with error: %v\nStdout: %s\nStderr: %s",
				err, stdout.String(), stderr.String())
		} else {
			log.Printf("✅ Filler completed successfully\nOutput: %s", stdout.String())
		}
	}()

	return true
}

// Verifica se a configuração fornecida consegue conectar e autenticar no IMAP.
func (a *App) VerifyIMAP(cfg SettingsDTO) bool {
	addr := cfg.ImapServer
	if addr == "" {
		addr = "imap.gmail.com:993"
	}

	log.Println("IMAP Verify: Dialing", addr)
	importClient := func() bool {
			// Mantém a lógica de conexão isolada neste helper.
		c, err := client.DialTLS(addr, nil)
		if err != nil {
			log.Printf("VerifyIMAP: Dial failed: %v", err)
			return false
		}
		defer c.Logout()

		if err := c.Login(cfg.ImapUser, cfg.ImapPass); err != nil {
			log.Printf("VerifyIMAP: Login failed: %v", err)
			return false
		}

		log.Println("VerifyIMAP: Success!")
		return true
	}

	return importClient()
}
