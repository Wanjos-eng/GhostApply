package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/emersion/go-imap/client"
	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/jung-kurt/gofpdf"
	"github.com/ledongthuc/pdf"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const defaultIMAPAddress = "imap.gmail.com:993"

var outboundHTTPClient = &http.Client{Timeout: 20 * time.Second}

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

type ProspectedJobDTO struct {
	ID        string `json:"id"`
	Titulo    string `json:"titulo"`
	Empresa   string `json:"empresa"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	Fonte     string `json:"fonte"`
	Descricao string `json:"descricao"`
	CriadoEm  string `json:"criado_em"`
}

type ProspectedMetricsDTO struct {
	TotalProspected int            `json:"total_prospected"`
	PendingCount    int            `json:"pending_count"`
	AnalyzedCount   int            `json:"analyzed_count"`
	RejectedCount   int            `json:"rejected_count"`
	DiscardedCount  int            `json:"discarded_count"`
	ManualCount     int            `json:"manual_count"`
	BySource        map[string]int `json:"by_source"`
	BySourceLast24h map[string]int `json:"by_source_last_24h"`
	ByStatus        map[string]int `json:"by_status"`
}

type PerformanceSuiteDTO struct {
	RanAt             string  `json:"ran_at"`
	Samples           int     `json:"samples"`
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
	HistoryRows       int     `json:"history_rows"`
	EmailRows         int     `json:"email_rows"`
	InterviewRows     int     `json:"interview_rows"`
	TotalSuiteMS      float64 `json:"total_suite_ms"`
	DatabaseReachable bool    `json:"database_reachable"`
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

func inferSourceFromURL(rawURL string) string {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(u, "linkedin.com"):
		return "linkedin"
	case strings.Contains(u, "gupy.io"):
		return "gupy"
	case strings.Contains(u, "greenhouse.io") || strings.Contains(u, "boards.greenhouse"):
		return "greenhouse"
	case strings.Contains(u, "lever.co") || strings.Contains(u, "jobs.lever"):
		return "lever"
	default:
		return "other"
	}
}

func parseCreatedAt(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		ts, err := time.Parse(layout, trimmed)
		if err == nil {
			return ts.UTC(), true
		}
	}

	return time.Time{}, false
}

func buildGeminiRequest(model, apiKey string, payload []byte) (*http.Request, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	return req, nil
}

func mergeSettingsEnv(existing map[string]string, cfg SettingsDTO) map[string]string {
	envMap := make(map[string]string, len(existing)+6)
	for k, v := range existing {
		envMap[k] = v
	}

	setSecret := func(key, value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			if _, ok := envMap[key]; ok {
				return
			}
			envMap[key] = ""
			return
		}
		envMap[key] = value
	}

	envMap["IMAP_SERVER"] = strings.TrimSpace(cfg.ImapServer)
	envMap["IMAP_USER"] = strings.TrimSpace(cfg.ImapUser)
	if ats := strings.TrimSpace(cfg.ATSMinScore); ats != "" {
		envMap["ATS_MIN_SCORE"] = ats
	}
	setSecret("COHERE_API_KEY", cfg.CohereAPIKey)
	setSecret("GROQ_API_KEY", cfg.GroqAPIKey)
	setSecret("GEMINI_API_KEY", cfg.GeminiAPIKey)
	setSecret("IMAP_PASS", cfg.ImapPass)

	return envMap
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func ensurePrivateFile(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := os.Chmod(path, 0o600); err != nil {
		log.Printf("privacy: aviso ao aplicar permissão 600 em %s: %v", path, err)
	}
}

func runRetentionPolicies(database *sql.DB, now time.Time, retentionDays int, maxEmails int) error {
	if database == nil {
		return nil
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	threshold := now.UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)

	if _, err := database.Exec("DELETE FROM Candidatura_Forjada WHERE criado_em < ?", threshold); err != nil {
		return fmt.Errorf("retention candidatura: %w", err)
	}
	if _, err := database.Exec("DELETE FROM Vaga_Prospectada WHERE criado_em < ?", threshold); err != nil {
		return fmt.Errorf("retention vaga: %w", err)
	}

	if maxEmails > 0 {
		if _, err := database.Exec(
			`DELETE FROM Email_Recrutador
			 WHERE id NOT IN (
			   SELECT id FROM Email_Recrutador ORDER BY rowid DESC LIMIT ?
			 )`,
			maxEmails,
		); err != nil {
			return fmt.Errorf("retention email: %w", err)
		}
	}

	return nil
}

// Estrutura principal da aplicação Wails.
type App struct {
	ctx                context.Context
	database           *sql.DB
	databasePath       string
	databaseStartupErr string
	pipelineMu         sync.RWMutex
	pipelineStatus     PipelineStatusDTO
}

type PipelineStepDTO struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

type PipelineStatusDTO struct {
	State      string            `json:"state"`
	Summary    string            `json:"summary"`
	StartedAt  string            `json:"started_at"`
	UpdatedAt  string            `json:"updated_at"`
	FinishedAt string            `json:"finished_at"`
	Steps      []PipelineStepDTO `json:"steps"`
	Logs       []string          `json:"logs"`
}

// Cria a instância principal do app.
func NewApp() *App {
	return &App{pipelineStatus: initialPipelineStatus()}
}

func initialPipelineStatus() PipelineStatusDTO {
	return PipelineStatusDTO{
		State:   "idle",
		Summary: "Pipeline parado",
		Steps: []PipelineStepDTO{
			{ID: "collect", Title: "Coleta de vagas", Status: "pending", Detail: "Aguardando início"},
			{ID: "triage", Title: "Triagem e elegibilidade", Status: "pending", Detail: "Aguardando coleta"},
			{ID: "forge", Title: "Melhoria de currículo", Status: "pending", Detail: "Aguardando triagem"},
			{ID: "apply", Title: "Candidatura automática", Status: "pending", Detail: "Aguardando melhoria de currículo"},
		},
		Logs: []string{"Pronto para iniciar."},
	}
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (a *App) clonePipelineStatusLocked() PipelineStatusDTO {
	cloned := a.pipelineStatus
	cloned.Steps = append([]PipelineStepDTO(nil), a.pipelineStatus.Steps...)
	cloned.Logs = append([]string(nil), a.pipelineStatus.Logs...)
	return cloned
}

func (a *App) appendPipelineLogLocked(message string) {
	timestamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message)
	a.pipelineStatus.Logs = append(a.pipelineStatus.Logs, timestamped)
	if len(a.pipelineStatus.Logs) > 120 {
		a.pipelineStatus.Logs = a.pipelineStatus.Logs[len(a.pipelineStatus.Logs)-120:]
	}
	a.pipelineStatus.UpdatedAt = nowISO()
}

func (a *App) setPipelineStepLocked(stepID, status, detail string) {
	for i := range a.pipelineStatus.Steps {
		if a.pipelineStatus.Steps[i].ID != stepID {
			continue
		}
		a.pipelineStatus.Steps[i].Status = status
		if strings.TrimSpace(detail) != "" {
			a.pipelineStatus.Steps[i].Detail = detail
		}
		now := nowISO()
		if status == "running" {
			a.pipelineStatus.Steps[i].StartedAt = now
		}
		if status == "done" || status == "error" {
			a.pipelineStatus.Steps[i].FinishedAt = now
		}
		a.pipelineStatus.UpdatedAt = now
		break
	}
}

// findProjectRoot walks upward from startDir until it finds a directory that
// contains the given sentinel sub-path (file or directory).  Returns the
// absolute path of the matching directory, or "" if none is found.
func findProjectRoot(startDir, sentinel string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, sentinel)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding the sentinel.
			break
		}
		dir = parent
	}
	return ""
}

// resolveProjectRootFor returns the absolute path of the repository root that
// contains the given sentinelDir sub-path (e.g. "cmd/scraper").
// It first searches upward from the running executable's directory, then from
// the process working directory, and finally falls back to ".." (the original
// relative-path behaviour) so that wails dev from dashboard/ still works.
func resolveProjectRootFor(sentinelDir string) string {
	if exePath, err := os.Executable(); err == nil {
		if root := findProjectRoot(filepath.Dir(exePath), sentinelDir); root != "" {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if root := findProjectRoot(cwd, sentinelDir); root != "" {
			return root
		}
	}
	// Last-resort: original relative-path behaviour (works when CWD=dashboard/).
	return ".."
}

func resolveScraperCommand() (*exec.Cmd, error) {
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, "scraper.exe"),
			filepath.Join(exeDir, "scraper"),
		}
		for _, candidate := range candidates {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				cmd := exec.Command(candidate)
				cmd.Dir = exeDir
				return cmd, nil
			}
		}
	}

	if _, lookErr := exec.LookPath("go"); lookErr == nil {
		projectRoot := resolveProjectRootFor(filepath.Join("cmd", "scraper"))
		cmd := exec.Command("go", "run", "./cmd/scraper")
		cmd.Dir = projectRoot
		return cmd, nil
	}

	return nil, fmt.Errorf("scraper binary not found and 'go' is unavailable")
}

func runBackgroundCommand(cmd *exec.Cmd) (string, error) {
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	combined := strings.TrimSpace(strings.Join([]string{out, errOut}, "\n"))
	if len(combined) > 800 {
		combined = combined[:800] + "..."
	}
	return combined, err
}

func truncateForPrompt(raw string, max int) string {
	if max <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max]
}

func extractPDFText(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, b)
	return strings.TrimSpace(buf.String()), nil
}

func generateTailoredResumeText(apiKey, baseCVText, jobDescription string) (string, error) {
	base := truncateForPrompt(baseCVText, 24000)
	job := truncateForPrompt(jobDescription, 10000)

	prompt := "" +
		"Você é um especialista em currículo ATS e revisão técnica para recrutadores de tecnologia.\n" +
		"Objetivo: maximizar aderência ATS SEM perder formato profissional humano.\n" +
		"Regras obrigatórias:\n" +
		"1) Texto simples, sem markdown, sem tabela, sem colunas, sem ícones, sem emojis, sem caracteres decorativos.\n" +
		"2) Não inventar fatos. Use somente dados plausíveis presentes no currículo base.\n" +
		"3) Espelhar naturalmente palavras-chave da vaga (skills, stack, ferramentas, senioridade, domínio).\n" +
		"4) Experiências em ordem cronológica reversa com bullets de impacto mensurável quando possível.\n" +
		"5) Linguagem objetiva, orientada a resultado, legível para recrutador em 20-30 segundos.\n" +
		"6) Evitar blocos longos; preferir linhas curtas e bullets.\n" +
		"Saída obrigatória exatamente nesta estrutura de seções:\n" +
		"NOME COMPLETO\n" +
		"CARGO ALVO\n" +
		"CONTATO\n" +
		"RESUMO PROFISSIONAL:\n" +
		"COMPETENCIAS TECNICAS:\n" +
		"EXPERIENCIA PROFISSIONAL:\n" +
		"PROJETOS RELEVANTES:\n" +
		"FORMACAO:\n" +
		"CERTIFICACOES (SE HOUVER):\n" +
		"IDIOMAS (SE HOUVER):\n\n" +
		"--- CURRICULO BASE ---\n" + base + "\n\n" +
		"--- DESCRICAO DA VAGA ---\n" + job

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{{"text": prompt}},
			},
		},
	}

	jsonValue, _ := json.Marshal(requestBody)
	req, reqErr := buildGeminiRequest("gemini-2.0-flash", apiKey, jsonValue)
	if reqErr != nil {
		return "", reqErr
	}

	resp, httpErr := outboundHTTPClient.Do(req)
	if httpErr != nil {
		return "", httpErr
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
		return "", decErr
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta Gemini vazia")
	}

	out := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)
	if out == "" {
		return "", fmt.Errorf("resposta Gemini sem conteúdo")
	}
	return out, nil
}

func isResumeHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasSuffix(trimmed, ":") {
		return true
	}
	headingMap := map[string]bool{
		"NOME COMPLETO":            true,
		"CARGO ALVO":               true,
		"CONTATO":                  true,
		"RESUMO PROFISSIONAL":      true,
		"COMPETENCIAS TECNICAS":    true,
		"EXPERIENCIA PROFISSIONAL": true,
		"PROJETOS RELEVANTES":      true,
		"FORMACAO":                 true,
		"CERTIFICACOES":            true,
		"IDIOMAS":                  true,
	}

	normalized := strings.TrimSuffix(trimmed, ":")
	return headingMap[normalized]
}

func writeResumePDF(filePath, content string) error {
	pdfDoc := gofpdf.New("P", "mm", "A4", "")
	pdfDoc.SetMargins(15, 15, 15)
	pdfDoc.SetAutoPageBreak(true, 15)
	pdfDoc.AddPage()
	pdfDoc.SetFont("Arial", "", 10.5)

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			pdfDoc.Ln(2.4)
			continue
		}

		if isResumeHeading(trimmed) {
			pdfDoc.SetFont("Arial", "B", 11.5)
			pdfDoc.MultiCell(0, 6.0, strings.TrimSuffix(trimmed, ":")+":", "", "L", false)
			pdfDoc.SetFont("Arial", "", 10.5)
			continue
		}

		pdfDoc.MultiCell(0, 5.0, trimmed, "", "L", false)
	}

	return pdfDoc.OutputFileAndClose(filePath)
}

func persistBaseCVPath(filePath string) {
	envPath := getAppEnvPath()
	existing, err := godotenv.Read(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("persistBaseCVPath: falha ao ler .env: %v", err)
		return
	}
	if existing == nil {
		existing = map[string]string{}
	}

	existing["BASE_CV_PATH"] = strings.TrimSpace(filePath)

	if writeErr := godotenv.Write(existing, envPath); writeErr != nil {
		log.Printf("persistBaseCVPath: falha ao escrever .env: %v", writeErr)
		return
	}
	_ = os.Chmod(envPath, 0o600)
	_ = os.Setenv("BASE_CV_PATH", strings.TrimSpace(filePath))
}

func parseATSMinimumScore(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0.40
	}

	val, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0.40
	}

	// Suporta valor em percentual inteiro (ex.: 65 -> 65%).
	if val > 1 {
		val = val / 100.0
	}

	if val < 0 {
		return 0
	}
	if val > 1 {
		return 1
	}
	return val
}

func tokenizeATS(text string) []string {
	stop := map[string]struct{}{
		"de": {}, "da": {}, "do": {}, "das": {}, "dos": {}, "para": {}, "com": {}, "sem": {},
		"and": {}, "the": {}, "for": {}, "with": {}, "this": {}, "that": {}, "from": {}, "into": {},
		"em": {}, "na": {}, "no": {}, "nas": {}, "nos": {}, "por": {}, "uma": {}, "um": {}, "as": {}, "os": {},
	}

	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return false
		}
		switch r {
		case '+', '#', '.', '-':
			return false
		default:
			return true
		}
	})

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.Trim(p, ".-_")
		if t == "" {
			continue
		}
		if len(t) < 3 {
			continue
		}
		if _, isStop := stop[t]; isStop {
			continue
		}
		tokens = append(tokens, t)
	}

	return tokens
}

func extractATSKeywords(jobDescription string, maxKeywords int) []string {
	if maxKeywords <= 0 {
		return nil
	}

	freq := map[string]int{}
	for _, t := range tokenizeATS(jobDescription) {
		freq[t]++
	}

	type kv struct {
		Token string
		Count int
	}
	items := make([]kv, 0, len(freq))
	for token, count := range freq {
		items = append(items, kv{Token: token, Count: count})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Token < items[j].Token
		}
		return items[i].Count > items[j].Count
	})

	if len(items) > maxKeywords {
		items = items[:maxKeywords]
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Token)
	}
	return out
}

func computeATSCoverage(resumeContent string, keywords []string) (float64, int, int, []string) {
	if len(keywords) == 0 {
		return 1, 0, 0, nil
	}

	resumeSet := map[string]struct{}{}
	for _, t := range tokenizeATS(resumeContent) {
		resumeSet[t] = struct{}{}
	}

	matched := 0
	missing := make([]string, 0)
	for _, kw := range keywords {
		if _, ok := resumeSet[kw]; ok {
			matched++
		} else {
			missing = append(missing, kw)
		}
	}

	score := float64(matched) / float64(len(keywords))
	return score, matched, len(keywords), missing
}

func (a *App) runInlineForger(cfg ProfileDTO) (int, int, int, error) {
	if a.database == nil {
		return 0, 0, 0, fmt.Errorf("banco indisponível")
	}

	_ = godotenv.Load(getAppEnvPath())
	baseCVPath := strings.TrimSpace(os.Getenv("BASE_CV_PATH"))
	if baseCVPath == "" {
		return 0, 0, 0, fmt.Errorf("CV base não configurado; faça upload do PDF no BaseProfile")
	}

	baseCVText, err := extractPDFText(baseCVPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("falha ao ler CV base: %w", err)
	}
	if baseCVText == "" {
		return 0, 0, 0, fmt.Errorf("CV base sem texto legível")
	}

	limit := cfg.AppsPerDay
	if limit <= 0 {
		limit = 50
	}

	rows, err := a.database.Query(`
		SELECT id, COALESCE(descricao, '')
		FROM Vaga_Prospectada
		WHERE status = 'PENDENTE'
		ORDER BY criado_em ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("falha ao carregar vagas pendentes: %w", err)
	}
	defer rows.Close()

	type forgeTarget struct {
		VagaID    string
		Descricao string
	}

	targets := make([]forgeTarget, 0, limit)
	for rows.Next() {
		var t forgeTarget
		if scanErr := rows.Scan(&t.VagaID, &t.Descricao); scanErr != nil {
			return 0, 0, 0, fmt.Errorf("falha ao ler vaga pendente: %w", scanErr)
		}
		targets = append(targets, t)
	}

	if len(targets) == 0 {
		return 0, 0, 0, nil
	}

	outputDir := filepath.Join(GetAppDir(), "forged-cv")
	if mkErr := os.MkdirAll(outputDir, 0o700); mkErr != nil {
		return 0, 0, 0, fmt.Errorf("falha ao preparar diretório de currículos: %w", mkErr)
	}

	geminiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	minATS := parseATSMinimumScore(os.Getenv("ATS_MIN_SCORE"))
	forgedCount := 0
	skippedCount := 0
	lowATScount := 0

	for _, target := range targets {
		var existingID string
		err := a.database.QueryRow(`
			SELECT id
			FROM Candidatura_Forjada
			WHERE vaga_id = ?
			  AND status IN ('FORJADO','ENVIADA','APLICADA','CONFIRMADA')
			LIMIT 1
		`, target.VagaID).Scan(&existingID)
		if err == nil {
			skippedCount++
			_, _ = a.database.Exec("UPDATE Vaga_Prospectada SET status = 'ANALISADA' WHERE id = ?", target.VagaID)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return forgedCount, skippedCount, lowATScount, fmt.Errorf("falha ao verificar candidatura existente para vaga %s: %w", target.VagaID, err)
		}

		resumeContent := baseCVText
		if geminiKey != "" && strings.TrimSpace(target.Descricao) != "" {
			tailored, genErr := generateTailoredResumeText(geminiKey, baseCVText, target.Descricao)
			if genErr == nil && strings.TrimSpace(tailored) != "" {
				resumeContent = tailored
			} else {
				log.Printf("forger: fallback para CV base na vaga %s: %v", target.VagaID, genErr)
			}
		}

		keywords := extractATSKeywords(target.Descricao, 40)
		score, matched, total, missing := computeATSCoverage(resumeContent, keywords)
		if total > 0 && score < minATS {
			lowATScount++
			missingPreview := ""
			if len(missing) > 0 {
				limitMissing := len(missing)
				if limitMissing > 8 {
					limitMissing = 8
				}
				missingPreview = strings.Join(missing[:limitMissing], ",")
			}

			_, _ = a.database.Exec("UPDATE Vaga_Prospectada SET status = 'ALERTA_MANUAL' WHERE id = ?", target.VagaID)
			log.Printf("forger: ATS abaixo do mínimo na vaga %s (score=%.2f min=%.2f matched=%d/%d missing=%s)", target.VagaID, score, minATS, matched, total, missingPreview)
			continue
		}

		fileName := fmt.Sprintf("%s-%s.pdf", target.VagaID, uuid.NewString())
		forgedPath := filepath.Join(outputDir, fileName)

		if writeErr := writeResumePDF(forgedPath, resumeContent); writeErr != nil {
			return forgedCount, skippedCount, lowATScount, fmt.Errorf("falha ao gerar PDF forjado para vaga %s: %w", target.VagaID, writeErr)
		}

		candID := uuid.NewString()
		if _, insErr := a.database.Exec(`
			INSERT INTO Candidatura_Forjada (id, vaga_id, curriculo_path, status)
			VALUES (?, ?, ?, 'FORJADO')
		`, candID, target.VagaID, forgedPath); insErr != nil {
			return forgedCount, skippedCount, lowATScount, fmt.Errorf("falha ao inserir candidatura forjada para vaga %s: %w", target.VagaID, insErr)
		}

		if _, updErr := a.database.Exec("UPDATE Vaga_Prospectada SET status = 'ANALISADA' WHERE id = ?", target.VagaID); updErr != nil {
			return forgedCount, skippedCount, lowATScount, fmt.Errorf("falha ao atualizar status ANALISADA da vaga %s: %w", target.VagaID, updErr)
		}

		forgedCount++
	}

	return forgedCount, skippedCount, lowATScount, nil
}

// StartAutomationPipeline inicia coleta, triagem e candidatura em sequência com status detalhado para UI.
func (a *App) StartAutomationPipeline(cfg ProfileDTO) bool {
	a.pipelineMu.Lock()
	if a.pipelineStatus.State == "running" {
		a.appendPipelineLogLocked("Pipeline já está em execução.")
		a.pipelineMu.Unlock()
		return false
	}

	a.pipelineStatus = initialPipelineStatus()
	a.pipelineStatus.State = "running"
	a.pipelineStatus.Summary = "Iniciando pipeline..."
	a.pipelineStatus.StartedAt = nowISO()
	a.pipelineStatus.UpdatedAt = a.pipelineStatus.StartedAt
	a.appendPipelineLogLocked(fmt.Sprintf("Inicializando pipeline com %d roles e %d tecnologias.", len(cfg.TargetRoles), len(cfg.CoreStack)))
	a.pipelineMu.Unlock()

	go func() {
		finishWithError := func(stepID, detail string) {
			a.pipelineMu.Lock()
			a.setPipelineStepLocked(stepID, "error", detail)
			a.pipelineStatus.State = "error"
			a.pipelineStatus.Summary = detail
			a.pipelineStatus.FinishedAt = nowISO()
			a.appendPipelineLogLocked("Pipeline finalizado com erro: " + detail)
			a.pipelineMu.Unlock()
		}

		a.pipelineMu.Lock()
		a.setPipelineStepLocked("collect", "running", "Procurando vagas em fontes configuradas")
		a.pipelineStatus.Summary = "Coletando vagas..."
		a.appendPipelineLogLocked("Etapa 1/4: iniciando coleta de vagas.")
		a.pipelineMu.Unlock()

		scraperCmd, err := resolveScraperCommand()
		if err != nil {
			finishWithError("collect", "Não foi possível iniciar coleta: scraper indisponível")
			return
		}
		scraperOutput, scraperErr := runBackgroundCommand(scraperCmd)
		if scraperErr != nil {
			detail := "Coleta falhou"
			if strings.TrimSpace(scraperOutput) != "" {
				detail = detail + ": " + scraperOutput
			} else {
				detail = detail + ": " + scraperErr.Error()
			}
			finishWithError("collect", detail)
			return
		}

		a.pipelineMu.Lock()
		a.setPipelineStepLocked("collect", "done", "Coleta concluída")
		a.appendPipelineLogLocked("Coleta finalizada com sucesso.")
		a.setPipelineStepLocked("triage", "running", "Calculando elegibilidade para candidatura")
		a.pipelineStatus.Summary = "Processando triagem de candidaturas..."
		jobs := a.FetchProspectedJobs()
		metrics := a.GetProspectedMetrics()
		a.setPipelineStepLocked("triage", "done", fmt.Sprintf("%d vagas coletadas (%d pendentes)", len(jobs), metrics.PendingCount))
		a.appendPipelineLogLocked(fmt.Sprintf("Triagem concluída: %d vagas detectadas, %d pendentes para pipeline.", len(jobs), metrics.PendingCount))
		a.pipelineMu.Unlock()

		a.pipelineMu.Lock()
		a.setPipelineStepLocked("forge", "running", "Adaptando currículo para cada vaga elegível")
		a.pipelineStatus.Summary = "Forjando currículos por vaga..."
		a.appendPipelineLogLocked("Etapa 3/4: iniciando melhoria de currículo por vaga.")
		a.pipelineMu.Unlock()

		forgedCount, skippedCount, lowATScount, forgeErr := a.runInlineForger(cfg)
		if forgeErr != nil {
			finishWithError("forge", "Forja de currículo falhou: "+forgeErr.Error())
			return
		}

		a.pipelineMu.Lock()
		forgeDetail := fmt.Sprintf("%d currículos forjados (%d já existentes, %d abaixo do ATS mínimo)", forgedCount, skippedCount, lowATScount)
		if forgedCount == 0 && skippedCount == 0 && lowATScount == 0 {
			forgeDetail = "Nenhuma vaga pendente para forjar"
		}
		a.setPipelineStepLocked("forge", "done", forgeDetail)
		a.appendPipelineLogLocked("Forja concluída: " + forgeDetail)
		a.setPipelineStepLocked("apply", "running", "Executando candidatura automática")
		a.pipelineStatus.Summary = "Aplicando nas vagas elegíveis..."
		a.appendPipelineLogLocked("Etapa 4/4: iniciando envio automático de candidaturas.")
		a.pipelineMu.Unlock()

		fillerCmd, err := resolveFillerCommand()
		if err != nil {
			finishWithError("apply", "Não foi possível iniciar candidatura: filler indisponível")
			return
		}
		fillerOutput, fillerErr := runBackgroundCommand(fillerCmd)
		if fillerErr != nil {
			detail := "Candidatura falhou"
			if strings.TrimSpace(fillerOutput) != "" {
				detail = detail + ": " + fillerOutput
			} else {
				detail = detail + ": " + fillerErr.Error()
			}
			finishWithError("apply", detail)
			return
		}

		a.pipelineMu.Lock()
		a.setPipelineStepLocked("apply", "done", "Candidatura automática concluída")
		a.pipelineStatus.State = "done"
		a.pipelineStatus.Summary = "Pipeline concluído com sucesso"
		a.pipelineStatus.FinishedAt = nowISO()
		a.appendPipelineLogLocked("Pipeline concluído com sucesso.")
		if strings.TrimSpace(fillerOutput) != "" {
			a.appendPipelineLogLocked("Resumo do filler: " + fillerOutput)
		}
		a.pipelineMu.Unlock()
	}()

	return true
}

func (a *App) GetAutomationPipelineStatus() PipelineStatusDTO {
	a.pipelineMu.RLock()
	defer a.pipelineMu.RUnlock()
	return a.clonePipelineStatusLocked()
}

// GetAppDir returns the persistent configuration directory (~/.ghostapply)
func GetAppDir() string {
	baseDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(baseDir) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || strings.TrimSpace(home) == "" {
			baseDir = "."
		} else {
			baseDir = home
		}
	}
	appDir := filepath.Join(baseDir, "GhostApply")
	_ = os.MkdirAll(appDir, 0o700)
	return appDir
}

func getAppEnvPath() string {
	return filepath.Join(GetAppDir(), ".env")
}

func ensureBootstrapEnv(appEnvPath string) {
	if _, err := os.Stat(appEnvPath); err == nil {
		return
	}

	defaults := map[string]string{
		"DATABASE_URL":        "forja_ghost.sqlite",
		"SESSION_PATH":        "session.json",
		"ATS_MIN_SCORE":       "0.40",
		"DATA_RETENTION_DAYS": "90",
		"EMAIL_RETENTION_MAX": "2000",
	}

	if err := godotenv.Write(defaults, appEnvPath); err != nil {
		log.Printf("WAILS: falha ao gerar .env padrão em %s: %v", appEnvPath, err)
		return
	}
	if err := os.Chmod(appEnvPath, 0o600); err != nil {
		log.Printf("WAILS: aviso ao aplicar permissão no .env padrão: %v", err)
	}
}

func resolveFillerCommand() (*exec.Cmd, error) {
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, "filler.exe"),
			filepath.Join(exeDir, "filler"),
		}
		for _, candidate := range candidates {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				cmd := exec.Command(candidate)
				cmd.Dir = exeDir
				return cmd, nil
			}
		}
	}

	if _, lookErr := exec.LookPath("go"); lookErr == nil {
		projectRoot := resolveProjectRootFor(filepath.Join("cmd", "filler"))
		cmd := exec.Command("go", "run", "./cmd/filler")
		cmd.Dir = projectRoot
		return cmd, nil
	}

	return nil, fmt.Errorf("filler binary not found and 'go' is unavailable")
}

func normalizeDatabasePath(rawPath, appDir string) string {
	trimmed := strings.Trim(strings.TrimSpace(rawPath), "\"'")
	if trimmed == "" {
		return filepath.Clean(filepath.Join(appDir, "forja_ghost.sqlite"))
	}
	if trimmed == ":memory:" {
		return trimmed
	}

	// Compatibilidade com formatos legados como:
	// - file:forja_ghost.sqlite?_pragma=...
	// - file:///C:/Users/.../forja_ghost.sqlite
	legacy := trimmed
	if strings.HasPrefix(strings.ToLower(legacy), "file:") {
		legacy = legacy[len("file:"):]
		legacy = strings.TrimPrefix(legacy, "//")
	}
	if q := strings.Index(legacy, "?"); q >= 0 {
		legacy = legacy[:q]
	}
	if strings.TrimSpace(legacy) == "" {
		legacy = "forja_ghost.sqlite"
	}

	abs := legacy
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(appDir, abs)
	}
	clean := filepath.Clean(abs)
	if resolved, err := filepath.Abs(clean); err == nil {
		return resolved
	}
	return clean
}

func legacyDatabaseCandidates(appDir string) []string {
	candidates := make([]string, 0, 3)
	seen := map[string]struct{}{}
	push := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		clean := filepath.Clean(trimmed)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		candidates = append(candidates, clean)
	}

	push(filepath.Join(appDir, "forja_ghost.sqlite"))

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		push(filepath.Join(home, ".ghostapply", "forja_ghost.sqlite"))
		push(filepath.Join(home, "GhostApply", "forja_ghost.sqlite"))
	}

	return candidates
}

func firstExistingFile(paths []string) string {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func resolvePreferredDatabasePath(rawPath, appDir string) string {
	normalized := normalizeDatabasePath(rawPath, appDir)
	if normalized == ":memory:" {
		return normalized
	}

	defaultPath := filepath.Join(appDir, "forja_ghost.sqlite")
	if normalized == defaultPath {
		if _, err := os.Stat(normalized); err == nil {
			return normalized
		}

		candidates := legacyDatabaseCandidates(appDir)
		legacy := firstExistingFile(candidates)
		if legacy != "" {
			if legacy != normalized {
				log.Printf("WAILS: banco legado detectado, usando %s", legacy)
			}
			return legacy
		}
	}

	return normalized
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return dst.Sync()
}

func mirrorSQLiteArtifacts(srcMainDBPath, dstMainDBPath string) {
	if strings.TrimSpace(srcMainDBPath) == "" || strings.TrimSpace(dstMainDBPath) == "" {
		return
	}
	if srcMainDBPath == dstMainDBPath {
		return
	}

	for _, ext := range []string{"", "-wal", "-shm"} {
		src := srcMainDBPath + ext
		dst := dstMainDBPath + ext
		info, err := os.Stat(src)
		if err != nil || info.IsDir() {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			log.Printf("WAILS: aviso ao copiar artefato SQLite %s -> %s: %v", src, dst, err)
		}
	}
}

func canWriteInDir(dir string) bool {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return false
	}
	tmpFile, err := os.CreateTemp(trimmed, "ghostapply-write-check-*.tmp")
	if err != nil {
		return false
	}
	name := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(name)
	return true
}

func shortenErr(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 180 {
		return msg[:180] + "..."
	}
	return msg
}

func diagnoseDatabaseStatus(database *sql.DB, dbPath, startupErr string) (string, string) {
	if database == nil {
		if strings.TrimSpace(startupErr) != "" {
			return "✗ ERRO", "Falha na inicialização do banco: " + startupErr
		}
		return "✗ ERRO", "Conexão de banco indisponível"
	}

	if pingErr := database.Ping(); pingErr != nil {
		return "✗ ERRO", "Falha no ping do banco: " + shortenErr(pingErr)
	}

	if strings.TrimSpace(dbPath) == "" || dbPath == ":memory:" {
		return "✓ OK", "Banco em memória ou caminho não persistente"
	}

	info, statErr := os.Stat(dbPath)
	if statErr != nil {
		return "✗ ERRO", "Arquivo do banco não encontrado/acessível: " + shortenErr(statErr)
	}
	if info.IsDir() {
		return "✗ ERRO", "O caminho do banco aponta para um diretório, não para arquivo"
	}

	dir := filepath.Dir(dbPath)
	if !canWriteInDir(dir) {
		return "⚠ LIMITADO", "Diretório do banco sem permissão de escrita (WAL pode falhar)"
	}

	return "✓ OK", "Banco acessível e diretório com escrita OK"
}

func openDashboardDatabase(dbPath, dbKey string) (*sql.DB, error) {
	baseDSN := buildDashboardSQLiteDSN(dbPath)
	database, err := sql.Open("sqlite", baseDSN)
	if err != nil {
		return nil, fmt.Errorf("openDashboardDatabase: %w", err)
	}

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("openDashboardDatabase: ping failed: %w", err)
	}

	if _, err := database.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("openDashboardDatabase: enable foreign keys: %w", err)
	}
	if dbPath != ":memory:" {
		if _, err := database.Exec("PRAGMA journal_mode = WAL;"); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("openDashboardDatabase: enable WAL: %w", err)
		}
	}

	if strings.TrimSpace(dbKey) != "" {
		log.Printf("WAILS: aviso — DB_ENCRYPTION_KEY configurada, mas o driver atual usa SQLite puro sem criptografia")
	}

	return database, nil
}

func buildDashboardSQLiteDSN(dbPath string) string {
	normalizedPath := strings.ReplaceAll(filepath.ToSlash(dbPath), "\\", "/")
	return fmt.Sprintf("file:%s", normalizedPath)
}

// Executa a inicialização quando o app sobe.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	appDir := GetAppDir()
	appEnvPath := filepath.Join(appDir, ".env")
	ensureBootstrapEnv(appEnvPath)

	// 1. Carrega configuração da pasta persistente do usuário.
	if err := godotenv.Load(appEnvPath); err != nil {
		// Compatibilidade com execução de desenvolvimento pela raiz do projeto.
		_ = godotenv.Load("../.env")
	}

	dbPath := resolvePreferredDatabasePath(os.Getenv("DATABASE_URL"), appDir)
	dbKey := os.Getenv("DB_ENCRYPTION_KEY")
	a.databasePath = dbPath
	a.databaseStartupErr = ""
	_ = os.Setenv("DATABASE_URL", dbPath)

	sessionPath := envOrDefault("SESSION_PATH", "session.json")
	if !filepath.IsAbs(sessionPath) {
		sessionPath = filepath.Join(appDir, sessionPath)
	}
	_ = os.Setenv("SESSION_PATH", sessionPath)

	ensurePrivateFile(appEnvPath)
	ensurePrivateFile(sessionPath)
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			log.Printf("privacy: aviso ao garantir diretório do banco: %v", err)
		}
		ensurePrivateFile(dbPath)
	}

	database, err := openDashboardDatabase(dbPath, dbKey)
	if err != nil && dbPath != filepath.Join(appDir, "forja_ghost.sqlite") {
		fallbackPath := filepath.Join(appDir, "forja_ghost.sqlite")
		if mkErr := os.MkdirAll(filepath.Dir(fallbackPath), 0o700); mkErr != nil {
			log.Printf("WAILS: falha ao preparar fallback do banco: %v", mkErr)
		}
		// Se a origem está legível mas não gravável (ex.: Program Files no Windows),
		// espelha o banco para o diretório gravável do usuário antes de reabrir.
		mirrorSQLiteArtifacts(dbPath, fallbackPath)
		database, err = openDashboardDatabase(fallbackPath, dbKey)
		if err == nil {
			dbPath = fallbackPath
			_ = os.Setenv("DATABASE_URL", dbPath)
			log.Printf("WAILS: usando fallback de banco em %s", dbPath)
		}
	}

	if err != nil {
		a.databaseStartupErr = shortenErr(err)
		log.Printf("WAILS: falha ao abrir conexão com banco: %v\n", err)
	} else {
		// O driver SQLite puro Go pode falhar em multi-statements no CREATE.
		// Separamos para garantir a montagem tática inicial.
		schemas := []string{
			`CREATE TABLE IF NOT EXISTS Vaga_Prospectada (
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
			);`,

			`CREATE TABLE IF NOT EXISTS Candidatura_Forjada (
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
			);`,
			`CREATE TABLE IF NOT EXISTS Email_Recrutador (
				id TEXT PRIMARY KEY,
				email TEXT,
				nome TEXT,
				classificacao TEXT,
				corpo TEXT
			);`,
		}

		for _, schema := range schemas {
			_, initErr := database.Exec(schema)
			if initErr != nil {
				log.Printf("WAILS: falha ao inicializar schema: %v\n", initErr)
			}
		}

		a.database = database

		retentionDays := envIntOrDefault("DATA_RETENTION_DAYS", 90)
		maxEmails := envIntOrDefault("EMAIL_RETENTION_MAX", 2000)
		if retentionErr := runRetentionPolicies(a.database, time.Now(), retentionDays, maxEmails); retentionErr != nil {
			log.Printf("privacy: falha na retenção automática: %v", retentionErr)
		}
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

// Retorna a lista de vagas prospectadas com inferência de fonte por URL.
func (a *App) FetchProspectedJobs() []ProspectedJobDTO {
	if a.database == nil {
		return nil
	}

	rows, err := a.database.Query(`
		SELECT id, titulo, empresa, url, status, COALESCE(descricao, ''), criado_em
		FROM Vaga_Prospectada
		ORDER BY criado_em DESC
		LIMIT 1500
	`)
	if err != nil {
		log.Printf("FetchProspectedJobs: falha na query: %v", err)
		return nil
	}
	defer rows.Close()

	var results []ProspectedJobDTO
	for rows.Next() {
		var item ProspectedJobDTO
		if err := rows.Scan(&item.ID, &item.Titulo, &item.Empresa, &item.URL, &item.Status, &item.Descricao, &item.CriadoEm); err != nil {
			continue
		}
		item.Fonte = inferSourceFromURL(item.URL)
		results = append(results, item)
	}

	return results
}

// Agrega métricas de prospecção para exibição no painel operacional.
func (a *App) GetProspectedMetrics() ProspectedMetricsDTO {
	metrics := ProspectedMetricsDTO{
		BySource:        map[string]int{},
		BySourceLast24h: map[string]int{},
		ByStatus:        map[string]int{},
	}

	jobs := a.FetchProspectedJobs()
	metrics.TotalProspected = len(jobs)
	threshold := time.Now().UTC().Add(-24 * time.Hour)

	for _, job := range jobs {
		status := strings.ToUpper(strings.TrimSpace(job.Status))
		source := strings.ToLower(strings.TrimSpace(job.Fonte))

		metrics.BySource[source]++
		metrics.ByStatus[status]++
		if createdAt, ok := parseCreatedAt(job.CriadoEm); ok && (createdAt.Equal(threshold) || createdAt.After(threshold)) {
			metrics.BySourceLast24h[source]++
		}

		switch status {
		case "PENDENTE":
			metrics.PendingCount++
		case "ANALISADA", "FORJADO":
			metrics.AnalyzedCount++
		case "REJEITADO_PRESENCIAL":
			metrics.RejectedCount++
		case "DESCARTADA":
			metrics.DiscardedCount++
		case "ALERTA_MANUAL":
			metrics.ManualCount++
		}
	}

	return metrics
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
	defer func() {
		if imapClient != nil && imapClient.client != nil {
			_ = imapClient.client.Logout()
		}
	}()

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

	req, err := buildGeminiRequest("gemini-2.0-flash", apiKey, jsonValue)
	if err != nil {
		return fmt.Sprintf("Falha ao montar requisição Gemini: %v", err)
	}

	resp, err := outboundHTTPClient.Do(req)
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
		"database":        "✓ OK",
		"database_detail": "",
		"database_path":   a.databasePath,
		"cohere":          "✗ OFFLINE",
		"groq":            "✗ OFFLINE",
		"gemini":          "✗ OFFLINE",
		"imap":            "✗ OFFLINE",
	}

	// Banco local com diagnóstico detalhado para facilitar troubleshooting na sidebar.
	dbState, dbDetail := diagnoseDatabaseStatus(a.database, a.databasePath, a.databaseStartupErr)
	status["database"] = dbState
	status["database_detail"] = dbDetail

	// Cohere
	cohere := NewCohereClient()
	if cohere.apiKey != "" {
		// Faz um teste rápido autenticado na API para validar conectividade.
		req, err := http.NewRequest("GET", "https://api.cohere.ai/v1/models", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+cohere.apiKey)
			req.Header.Set("Accept", "application/json")
			resp, err := cohere.client.Do(req)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 500 {
				status["cohere"] = "✓ OK"
				resp.Body.Close()
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// Groq
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		req, err := http.NewRequest("GET", "https://api.groq.com/openai/v1/models", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+groqKey)
			resp, err := outboundHTTPClient.Do(req)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 500 {
				status["groq"] = "✓ OK"
				resp.Body.Close()
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// Gemini
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey != "" {
		probe := []byte(`{"contents":[{"parts":[{"text":"ping"}]}]}`)
		req, err := buildGeminiRequest("gemini-1.5-pro", geminiKey, probe)
		if err == nil {
			resp, err := outboundHTTPClient.Do(req)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 500 {
				status["gemini"] = "✓ OK"
				resp.Body.Close()
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// IMAP
	imapServer := os.Getenv("IMAP_SERVER")
	if imapServer != "" {
		listener, err := NewIMAPListener()
		if err == nil {
			status["imap"] = "✓ OK"
			if listener != nil && listener.client != nil {
				_ = listener.client.Logout()
			}
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
	ATSMinScore  string `json:"ats_min_score"`
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
	SourceFile     string   `json:"source_file"`
	ParseStatus    string   `json:"parse_status"`
}

// Carrega o mapeamento local do .env para a tela de configurações do frontend.
func (a *App) LoadSettings() SettingsDTO {
	_ = godotenv.Load(getAppEnvPath())
	return SettingsDTO{
		// Não devolve segredos para o frontend por padrão.
		CohereAPIKey: "",
		GroqAPIKey:   "",
		GeminiAPIKey: "",
		ATSMinScore:  envOrDefault("ATS_MIN_SCORE", "0.40"),
		ImapServer:   os.Getenv("IMAP_SERVER"),
		ImapUser:     os.Getenv("IMAP_USER"),
		ImapPass:     "",
	}
}

// Persiste os valores do mapa de volta no .env local de forma segura.
func (a *App) SaveSettings(cfg SettingsDTO) bool {
	envPath := getAppEnvPath()
	existing, err := godotenv.Read(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("SaveSettings: falha ao ler .env atual: %v", err)
		return false
	}

	envMap := mergeSettingsEnv(existing, cfg)

	// Persiste no .env local.
	err = godotenv.Write(envMap, envPath)
	if err != nil {
		log.Printf("SaveSettings: falha ao escrever .env: %v", err)
		return false
	}

	if chmodErr := os.Chmod(envPath, 0o600); chmodErr != nil {
		log.Printf("SaveSettings: aviso ao aplicar permissão restrita no .env: %v", chmodErr)
	}

	return true
}

// Abre o seletor nativo, extrai o texto do PDF e pede ao Gemini o JSON estruturado.
func (a *App) UploadAndParseCV() ProfileDTO {
	baseProfile := ProfileDTO{
		StrictlyRemote: true,
		MinSalaryFloor: "$120,000",
		AppsPerDay:     50,
		ParseStatus:    "idle",
	}

	// 1. Abre o diálogo do sistema operacional.
	filePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select your CV (PDF)",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "PDF Files (*.pdf)", Pattern: "*.pdf"},
		},
	})
	if err != nil || filePath == "" {
		log.Println("UploadAndParseCV: No file selected or error:", err)
		baseProfile.ParseStatus = "cancelled"
		return baseProfile
	}

	baseProfile.SourceFile = filepath.Base(filePath)
	baseProfile.ParseStatus = "uploaded"
	persistBaseCVPath(filePath)

	log.Println("Parsing PDF File:", filePath)

	// 2. Lê o texto do PDF.
	f, r, err := pdf.Open(filePath)
	if err != nil {
		log.Printf("UploadAndParseCV: Fail to open PDF: %v\n", err)
		baseProfile.ParseStatus = "error"
		return baseProfile
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		log.Printf("UploadAndParseCV: Fail to extract text: %v\n", err)
		baseProfile.ParseStatus = "error"
		return baseProfile
	}
	buf.ReadFrom(b)
	textContent := buf.String()

	// 3. Monta a requisição para o Gemini estruturar TargetRoles e CoreStack.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Println("UploadAndParseCV: No GEMINI_API_KEY found")
		baseProfile.ParseStatus = "uploaded"
		return baseProfile
	}

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
	req, reqErr := buildGeminiRequest("gemini-2.0-flash", apiKey, jsonValue)
	if reqErr != nil {
		log.Printf("UploadAndParseCV: Failed to build Gemini request: %v\n", reqErr)
		baseProfile.ParseStatus = "error"
		return baseProfile
	}

	resp, httpErr := outboundHTTPClient.Do(req)
	if httpErr != nil {
		log.Printf("UploadAndParseCV: Gemini HTTP Call Failed: %v\n", httpErr)
		baseProfile.ParseStatus = "error"
		return baseProfile
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
		baseProfile.ParseStatus = "error"
		return baseProfile
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		geminiJSON := result.Candidates[0].Content.Parts[0].Text
		geminiJSON = string(bytes.TrimSpace([]byte(geminiJSON)))
		log.Printf("UploadAndParseCV: Gemini JSON recebido (%d bytes)", len(geminiJSON))
		var parsed ProfileDTO
		parsed = baseProfile

		if err := json.Unmarshal([]byte(geminiJSON), &parsed); err != nil {
			log.Printf("UploadAndParseCV: Could not unmarshal string format: %v", err)
			baseProfile.ParseStatus = "uploaded"
			return baseProfile
		}

		parsed.SourceFile = baseProfile.SourceFile
		parsed.ParseStatus = "parsed"

		return parsed
	}

	baseProfile.ParseStatus = "uploaded"
	return baseProfile
}

// StartDaemon inicia o job batch do filler em segundo plano.
func (a *App) StartDaemon(cfg ProfileDTO) bool {
	log.Printf("🚀 WAILS: Launching Filler Daemon with config: %+v", cfg)

	cmd, err := resolveFillerCommand()
	if err != nil {
		log.Printf("❌ Filler launch failed (preflight): %v", err)
		return false
	}

	// Sobe o filler como subprocesso em background sem travar a UI.
	go func() {
		// Herda o ambiente atual; o filler lê o .env na inicialização.
		cmd.Env = os.Environ()

		// Captura stdout e stderr para diagnóstico se o processo encerrar cedo.
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			log.Printf("❌ Filler start failed: %v", err)
			return
		}

		// Aguarda término em background apenas para logging e diagnóstico.
		if err := cmd.Wait(); err != nil {
			log.Printf("❌ Filler exited with error: %v (stdout_bytes=%d stderr_bytes=%d)",
				err, stdout.Len(), stderr.Len())
		} else {
			log.Printf("✅ Filler completed successfully (stdout_bytes=%d)", stdout.Len())
		}
	}()

	return true
}

// Verifica se a configuração fornecida consegue conectar e autenticar no IMAP.
func (a *App) VerifyIMAP(cfg SettingsDTO) bool {
	addr := composeIMAPAddr(cfg.ImapServer, "")
	if strings.TrimSpace(cfg.ImapServer) == "" {
		addr = defaultIMAPAddress
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
