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

// App struct
type App struct {
	ctx      context.Context
	database *sql.DB
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
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
		// Initialize missing tables dynamically to prevent UI polling crash 
		_, initErr := database.Exec(`
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

	// Trigger background sync task
	go a.SyncEmailsRoutine()
}

// FetchEmails returns the emails parsed mapped for the Kanban GUI.
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

// FetchInterviews directly requests specifically "ENTREVISTA" entries for priority viewing.
func (a *App) FetchInterviews() []EmailRecrutador {
	var ints []EmailRecrutador
	for _, em := range a.FetchEmails() {
		if em.Classificacao == "ENTREVISTA" {
			ints = append(ints, em)
		}
	}
	return ints
}

// FetchHistory returns deeply nested applications + prospects across AI state lifecycle.
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

// SyncEmailsRoutine hooks into imap_listener.go and cohere.go 
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
		
		// Map mock logic persistence inside Email_Recrutador table
		pseudoUUID := fmt.Sprintf("email-%d", seqId)
		_, execErr := a.database.Exec("INSERT INTO Email_Recrutador (id, email, classificacao, corpo) VALUES (?, ?, ?, ?)",
			pseudoUUID, "recruiter@example.com", classificacao, body)
			
		if execErr == nil {
			imapClient.MarkAsSeen(seqId)
			log.Printf("SyncEmails: Appended message [%s]", classificacao)
		}
	}
}

// GenerateOutreachMessage chama o Cohere para fabricar mensagem de abordagem quente
func (a *App) GenerateOutreachMessage(recruiterName, roleName string) string {
	cohere := NewCohereClient()
	msg, err := cohere.GenerateOutreachMessage(recruiterName, roleName)
	if err != nil {
		return fmt.Sprintf("Erro ao gerar Outreach: %v", err)
	}
	return msg
}

// GerarDossieEstudos invokes Gemini against the body of an interview email
// Task 55: Dossier Generator overlay callback with Web Search Grounding
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

// ----------------------------------------------------
// Settings & Config Structs for UI Bindings
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
	TargetRoles     []string `json:"target_roles"`
	CoreStack       []string `json:"core_stack"`
	StrictlyRemote  bool     `json:"strictly_remote"`
	MinSalaryFloor  string   `json:"min_salary_floor"`
	AppsPerDay      int      `json:"apps_per_day"`
}

// LoadSettings reads the local .env mapping into the frontend Settings UI
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

// SaveSettings writes the map string values back down to the local .env securely
func (a *App) SaveSettings(cfg SettingsDTO) bool {
	envMap := map[string]string{
		"COHERE_API_KEY": cfg.CohereAPIKey,
		"GROQ_API_KEY":   cfg.GroqAPIKey,
		"GEMINI_API_KEY": cfg.GeminiAPIKey,
		"IMAP_SERVER":    cfg.ImapServer,
		"IMAP_USER":      cfg.ImapUser,
		"IMAP_PASS":      cfg.ImapPass,
	}

	// Persist to .env
	err := godotenv.Write(envMap, "../.env")
	if err != nil {
		log.Printf("SaveSettings: Failed to write .env: %v", err)
		return false
	}
	return true
}

// UploadAndParseCV invokes native file picker, extracts text from PDF and asks Gemini for JSON
func (a *App) UploadAndParseCV() ProfileDTO {
	// 1. Invoke OS Dialog
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

	// 2. Read PDF Text
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

	// 3. Setup Gemini Request to structure into TargetRoles / CoreStack
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
		// Fill defaults
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

// StartDaemon initializes the Playwright robot with the profile constraints
func (a *App) StartDaemon(cfg ProfileDTO) bool {
	log.Printf("🚀 WAILS: Launching Automation Daemon with config: %+v", cfg)
	
	// TODO: Spawn `exec.Command` or trigger internal `automation.go` goroutine 
	// specific to the Playwright core. 
	// For now, we simulate backend readiness to the UI:
	return true
}

// VerifyIMAP tests if the supplied configuration can connect and authenticate properly
func (a *App) VerifyIMAP(cfg SettingsDTO) bool {
	addr := cfg.ImapServer
	if addr == "" {
		addr = "imap.gmail.com:993"
	}
	
	log.Println("IMAP Verify: Dialing", addr)
	// Must locally import to avoid scope issues in this standalone function
	importClient := func() bool {
		// Import already global at file level: "github.com/emersion/go-imap/client"
		// Dialing TLS
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

