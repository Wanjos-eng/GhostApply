package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	"github.com/playwright-community/playwright-go"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const defaultIMAPAddress = "imap.gmail.com:993"
const defaultGeminiModel = "gemini-2.0-flash"

var outboundHTTPClient = &http.Client{Timeout: 20 * time.Second}

var geminiModelCache = struct {
	mu      sync.RWMutex
	apiKey  string
	model   string
	expires time.Time
}{expires: time.Time{}}

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
	if strings.TrimSpace(model) == "" {
		model = resolveGeminiModel(apiKey, false)
	}
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	return req, nil
}

func parseRetryAfterDelay(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds <= 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	if ts, err := time.Parse(time.RFC1123, trimmed); err == nil {
		d := time.Until(ts)
		if d > 0 {
			return d, true
		}
	}

	if ts, err := time.Parse(time.RFC1123Z, trimmed); err == nil {
		d := time.Until(ts)
		if d > 0 {
			return d, true
		}
	}

	return 0, false
}

func isRetryableGeminiStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func invalidateGeminiModelCache() {
	geminiModelCache.mu.Lock()
	geminiModelCache.apiKey = ""
	geminiModelCache.model = ""
	geminiModelCache.expires = time.Time{}
	geminiModelCache.mu.Unlock()
}

func supportsGenerateContent(methods []string) bool {
	for _, method := range methods {
		if strings.EqualFold(strings.TrimSpace(method), "generateContent") {
			return true
		}
	}
	return false
}

func resolveGeminiModel(apiKey string, forceRefresh bool) string {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" {
		return defaultGeminiModel
	}

	now := time.Now()
	if !forceRefresh {
		geminiModelCache.mu.RLock()
		if geminiModelCache.apiKey == trimmedKey && geminiModelCache.model != "" && now.Before(geminiModelCache.expires) {
			cached := geminiModelCache.model
			geminiModelCache.mu.RUnlock()
			return cached
		}
		geminiModelCache.mu.RUnlock()
	}

	req, err := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models?pageSize=100", nil)
	if err != nil {
		return defaultGeminiModel
	}
	req.Header.Set("x-goog-api-key", trimmedKey)

	resp, err := outboundHTTPClient.Do(req)
	if err != nil {
		log.Printf("resolveGeminiModel: list models failed: %v", err)
		return defaultGeminiModel
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	_ = resp.Body.Close()
	if readErr != nil {
		log.Printf("resolveGeminiModel: list models read failed: %v", readErr)
		return defaultGeminiModel
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 220 {
			preview = preview[:220] + "..."
		}
		log.Printf("resolveGeminiModel: list models HTTP %d: %s", resp.StatusCode, preview)
		return defaultGeminiModel
	}

	var listResponse struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&listResponse); err != nil {
		log.Printf("resolveGeminiModel: decode list models failed: %v", err)
		return defaultGeminiModel
	}

	selected := ""
	for _, model := range listResponse.Models {
		if !supportsGenerateContent(model.SupportedGenerationMethods) {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSpace(model.Name), "models/")
		if name == "" {
			continue
		}
		selected = name
		break
	}
	if selected == "" {
		selected = defaultGeminiModel
	}

	geminiModelCache.mu.Lock()
	geminiModelCache.apiKey = trimmedKey
	geminiModelCache.model = selected
	geminiModelCache.expires = now.Add(20 * time.Minute)
	geminiModelCache.mu.Unlock()

	return selected
}

func doGeminiRequestWithRetry(model, apiKey string, payload []byte, maxAttempts int, initialBackoff time.Duration) ([]byte, int, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if initialBackoff <= 0 {
		initialBackoff = 1200 * time.Millisecond
	}

	var lastErr error
	var lastBody []byte
	lastStatus := 0
	requestedModel := strings.TrimSpace(model)
	modelAuto := requestedModel == ""
	forceModelRefresh := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		effectiveModel := requestedModel
		if modelAuto {
			effectiveModel = resolveGeminiModel(apiKey, forceModelRefresh)
			forceModelRefresh = false
		}

		req, reqErr := buildGeminiRequest(effectiveModel, apiKey, payload)
		if reqErr != nil {
			return nil, 0, reqErr
		}

		resp, httpErr := outboundHTTPClient.Do(req)
		if httpErr != nil {
			lastErr = httpErr
			if attempt < maxAttempts {
				backoff := initialBackoff * time.Duration(1<<(attempt-1))
				if backoff > 12*time.Second {
					backoff = 12 * time.Second
				}
				time.Sleep(backoff)
				continue
			}
			return nil, 0, httpErr
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxAttempts {
				backoff := initialBackoff * time.Duration(1<<(attempt-1))
				if backoff > 12*time.Second {
					backoff = 12 * time.Second
				}
				time.Sleep(backoff)
				continue
			}
			return nil, resp.StatusCode, readErr
		}

		lastBody = body
		lastStatus = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, resp.StatusCode, nil
		}

		lastErr = fmt.Errorf("Gemini HTTP %d (model=%s)", resp.StatusCode, effectiveModel)
		if modelAuto && resp.StatusCode == http.StatusNotFound && attempt < maxAttempts {
			invalidateGeminiModelCache()
			forceModelRefresh = true
			continue
		}
		if !isRetryableGeminiStatus(resp.StatusCode) || attempt >= maxAttempts {
			break
		}

		if retryAfter, ok := parseRetryAfterDelay(resp.Header.Get("Retry-After")); ok {
			if retryAfter > 15*time.Second {
				retryAfter = 15 * time.Second
			}
			time.Sleep(retryAfter)
			continue
		}

		backoff := initialBackoff * time.Duration(1<<(attempt-1))
		if backoff > 12*time.Second {
			backoff = 12 * time.Second
		}
		time.Sleep(backoff)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("falha desconhecida na chamada Gemini")
	}
	return lastBody, lastStatus, lastErr
}

func mergeSettingsEnv(existing map[string]string, cfg SettingsDTO) map[string]string {
	envMap := make(map[string]string, len(existing)+11)
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
	envMap["SEARCH_KEYWORDS"] = strings.TrimSpace(cfg.SearchKeywords)
	envMap["SEARCH_COUNTRY"] = strings.TrimSpace(cfg.SearchCountry)
	envMap["GUPY_COMPANY_URLS"] = strings.TrimSpace(cfg.GupyBoards)
	envMap["GREENHOUSE_BOARDS"] = strings.TrimSpace(cfg.GreenhouseBoards)
	envMap["LEVER_COMPANIES"] = strings.TrimSpace(cfg.LeverCompanies)
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
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	dir := abs
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

func resolveProjectRootForStrict(sentinelDir string) (string, bool) {
	if exePath, err := os.Executable(); err == nil {
		if root := findProjectRoot(filepath.Dir(exePath), sentinelDir); root != "" {
			return root, true
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if root := findProjectRoot(cwd, sentinelDir); root != "" {
			return root, true
		}
	}
	return "", false
}

func resolveScraperCommand() (*exec.Cmd, error) {
	if _, lookErr := exec.LookPath("go"); lookErr == nil {
		if projectRoot, ok := resolveProjectRootForStrict(filepath.Join("cmd", "scraper")); ok {
			cmd := exec.Command("go", "run", "./cmd/scraper")
			cmd.Dir = projectRoot
			return cmd, nil
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		var candidates []string
		if runtime.GOOS == "windows" {
			candidates = []string{
				filepath.Join(exeDir, "scraper.exe"),
				filepath.Join(exeDir, "scraper"),
			}
		} else {
			candidates = []string{
				filepath.Join(exeDir, "scraper"),
				filepath.Join(exeDir, "scraper.exe"),
			}
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

	return nil, fmt.Errorf("scraper binary not found and project root for `go run ./cmd/scraper` was not detected")
}

func resolveProjectRootForGoRun() (string, bool) {
	candidates := make([]string, 0, 8)
	seen := map[string]struct{}{}
	push := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		candidates = append(candidates, clean)
	}

	if wd, err := os.Getwd(); err == nil {
		push(wd)
		push(filepath.Join(wd, ".."))
		push(filepath.Join(wd, "../.."))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		push(exeDir)
		push(filepath.Join(exeDir, ".."))
		push(filepath.Join(exeDir, "../.."))
	}

	for _, dir := range candidates {
		info, err := os.Stat(filepath.Join(dir, "cmd", "scraper", "main.go"))
		if err == nil && !info.IsDir() {
			return dir, true
		}
	}

	return "", false
}

func runBackgroundCommand(cmd *exec.Cmd) (string, error) {
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}
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

func runBackgroundCommandWithLogs(cmd *exec.Cmd, onLog func(string)) (string, error) {
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	var combinedMu sync.Mutex
	var combinedParts []string
	appendCombined := func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return
		}
		combinedMu.Lock()
		combinedParts = append(combinedParts, trimmed)
		if len(combinedParts) > 200 {
			combinedParts = combinedParts[len(combinedParts)-200:]
		}
		combinedMu.Unlock()
	}

	emit := func(line string) {
		appendCombined(line)
		if onLog != nil && strings.TrimSpace(line) != "" {
			onLog(line)
		}
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var wg sync.WaitGroup
	readPipe := func(prefix string, pipe io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		const maxLogLine = 1024 * 1024
		scanner.Buffer(make([]byte, 0, 64*1024), maxLogLine)
		for scanner.Scan() {
			line := scanner.Text()
			if prefix != "" {
				emit(prefix + line)
			} else {
				emit(line)
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			emit(prefix + "erro ao ler stream: " + scanErr.Error())
		}
	}

	wg.Add(2)
	go readPipe("", stdoutPipe)
	go readPipe("", stderrPipe)

	waitErr := cmd.Wait()
	wg.Wait()

	combinedMu.Lock()
	combined := strings.TrimSpace(strings.Join(combinedParts, "\n"))
	combinedMu.Unlock()
	if len(combined) > 2000 {
		combined = combined[:2000] + "..."
	}
	return combined, waitErr
}

func ensurePlaywrightRuntime() error {
	if _, err := playwright.Run(); err == nil {
		return nil
	}

	if installErr := playwright.Install(); installErr != nil {
		return fmt.Errorf("playwright install failed: %w", installErr)
	}

	pw, retryErr := playwright.Run()
	if retryErr != nil {
		return fmt.Errorf("playwright start failed after install: %w", retryErr)
	}
	_ = pw.Stop()
	return nil
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

func normalizeSearchTerms(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})

	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		term := strings.TrimSpace(part)
		if term == "" {
			continue
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, term)
	}

	return out
}

func buildScraperSearchKeywords(cfg ProfileDTO) string {
	seed := normalizeSearchTerms(os.Getenv("SEARCH_KEYWORDS"))
	seen := map[string]struct{}{}
	out := make([]string, 0, len(seed)+len(cfg.TargetRoles)+len(cfg.CoreStack)+len(cfg.SuggestedKeywords))

	appendTerm := func(term string) {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}

	for _, term := range seed {
		appendTerm(term)
	}
	for _, term := range cfg.SuggestedKeywords {
		appendTerm(term)
	}
	for _, term := range cfg.TargetRoles {
		appendTerm(term)
	}
	for _, term := range cfg.CoreStack {
		appendTerm(term)
	}

	return strings.Join(out, ",")
}

func sanitizeGeminiTerms(values []string, maxItems int) []string {
	if maxItems <= 0 || len(values) == 0 {
		return nil
	}

	initialCap := len(values)
	if initialCap > maxItems {
		initialCap = maxItems
	}
	out := make([]string, 0, initialCap)
	seen := map[string]struct{}{}
	for _, raw := range values {
		term := strings.TrimSpace(raw)
		if term == "" {
			continue
		}
		if len(term) > 80 {
			term = term[:80]
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, term)
		if len(out) >= maxItems {
			break
		}
	}

	return out
}

func sanitizeGeminiRationale(raw string, maxLen int) string {
	trimmed := strings.TrimSpace(raw)
	if maxLen <= 0 || trimmed == "" {
		return ""
	}
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen]
}

func sanitizeGeminiEnum(raw string, allowed map[string]struct{}, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return fallback
	}
	if _, ok := allowed[value]; !ok {
		return fallback
	}
	return value
}

func sanitizeGeminiSources(values []string) []string {
	allowed := map[string]struct{}{
		"linkedin":   {},
		"gupy":       {},
		"greenhouse": {},
		"lever":      {},
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		s := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := allowed[s]; !ok {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func extractJSONObjectFromText(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("resposta vazia")
	}

	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 {
			if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
				lines = lines[1:]
			}
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	start := strings.Index(trimmed, "{")
	if start < 0 {
		return "", fmt.Errorf("JSON não encontrado na resposta")
	}

	depth := 0
	for i := start; i < len(trimmed); i++ {
		switch trimmed[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(trimmed[start : i+1]), nil
			}
		}
	}

	return "", fmt.Errorf("JSON incompleto na resposta")
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
	body, statusCode, reqErr := doGeminiRequestWithRetry("", apiKey, jsonValue, 4, 1200*time.Millisecond)
	if reqErr != nil {
		return "", reqErr
	}
	if statusCode < 200 || statusCode >= 300 {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 220 {
			preview = preview[:220] + "..."
		}
		return "", fmt.Errorf("Gemini HTTP %d: %s", statusCode, preview)
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

	if decErr := json.NewDecoder(bytes.NewReader(body)).Decode(&result); decErr != nil {
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

func isManualReviewCompany(companyName string, jobURL string) bool {
	blob := strings.ToLower(strings.TrimSpace(companyName) + " " + strings.TrimSpace(jobURL))
	if blob == "" {
		return false
	}

	manualMarkers := []string{
		"nubank",
		"nu bank",
		"pagbank",
		"pagseguro",
		"itau",
		"itaú",
		"banco inter",
		"inter",
		"stone",
		"c6 bank",
		"c6",
		"santander",
		"bradesco",
		"bb",
		"banco do brasil",
		"caixa",
		"xp inc",
		"xp investimentos",
	}

	for _, marker := range manualMarkers {
		if strings.Contains(blob, marker) {
			return true
		}
	}

	return false
}

func (a *App) runInlineForger(cfg ProfileDTO) (int, int, int, error) {
	if a.database == nil {
		return 0, 0, 0, fmt.Errorf("banco indisponível")
	}

	_ = godotenv.Load(getAppEnvPath())
	baseCVPath := strings.TrimSpace(os.Getenv("BASE_CV_PATH"))
	if baseCVPath == "" {
		log.Printf("forger: CV base não configurado; etapa de forja ignorada nesta execução")
		return 0, 0, 0, nil
	}

	baseCVText, err := extractPDFText(baseCVPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("forger: CV base inexistente em %s; etapa de forja ignorada: %v", baseCVPath, err)
			return 0, 0, 0, nil
		}
		return 0, 0, 0, fmt.Errorf("falha ao ler CV base: %w", err)
	}
	if baseCVText == "" {
		log.Printf("forger: CV base sem texto legível; etapa de forja ignorada")
		return 0, 0, 0, nil
	}

	limit := cfg.AppsPerDay
	if limit <= 0 {
		limit = 50
	}

	nextGeminiAt := time.Time{}

	rows, err := a.database.Query(`
		SELECT id, COALESCE(titulo, ''), COALESCE(empresa, ''), COALESCE(url, ''), COALESCE(descricao, '')
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
		Titulo    string
		Empresa   string
		URL       string
		Descricao string
	}

	targets := make([]forgeTarget, 0, limit)
	for rows.Next() {
		var t forgeTarget
		if scanErr := rows.Scan(&t.VagaID, &t.Titulo, &t.Empresa, &t.URL, &t.Descricao); scanErr != nil {
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
			if !nextGeminiAt.IsZero() {
				if wait := time.Until(nextGeminiAt); wait > 0 {
					time.Sleep(wait)
				}
			}
			tailored, genErr := generateTailoredResumeText(geminiKey, baseCVText, target.Descricao)
			if genErr == nil && strings.TrimSpace(tailored) != "" {
				resumeContent = tailored
			} else {
				log.Printf("forger: fallback para CV base na vaga %s: %v", target.VagaID, genErr)
			}
			nextGeminiAt = time.Now().Add(1500 * time.Millisecond)
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

			if isManualReviewCompany(target.Empresa+" "+target.Titulo, target.URL) {
				_, _ = a.database.Exec("UPDATE Vaga_Prospectada SET status = 'ALERTA_MANUAL' WHERE id = ?", target.VagaID)
				log.Printf("forger: ATS abaixo do mínimo e empresa em revisão manual na vaga %s (score=%.2f min=%.2f matched=%d/%d missing=%s)", target.VagaID, score, minATS, matched, total, missingPreview)
				continue
			}

			if strings.TrimSpace(missingPreview) != "" {
				log.Printf("forger: ATS abaixo do mínimo, mas mantendo forja na vaga %s (score=%.2f min=%.2f matched=%d/%d missing=%s)", target.VagaID, score, minATS, matched, total, missingPreview)
			} else {
				log.Printf("forger: ATS abaixo do mínimo, mas mantendo forja na vaga %s (score=%.2f min=%.2f matched=%d/%d)", target.VagaID, score, minATS, matched, total)
			}
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

func (a *App) countReadyApplications() int {
	if a.database == nil {
		return 0
	}
	var total int
	if err := a.database.QueryRow(`SELECT COUNT(1) FROM Candidatura_Forjada WHERE status = 'FORJADO'`).Scan(&total); err != nil {
		return 0
	}
	return total
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
	a.appendPipelineLogLocked(fmt.Sprintf("Gemini indicou %d palavras-chave de busca.", len(cfg.SuggestedKeywords)))
	a.appendPipelineLogLocked(fmt.Sprintf("Estratégia Gemini: senioridade=%s remoto=%s fontes=%d exclusões=%d.", cfg.SuggestedSeniority, cfg.SuggestedRemotePolicy, len(cfg.SuggestedSources), len(cfg.SuggestedExcludeKeywords)))
	appsPerDay := cfg.AppsPerDay
	if appsPerDay <= 0 {
		appsPerDay = 50
	}
	scraperCollectCap := appsPerDay * 2
	a.appendPipelineLogLocked(fmt.Sprintf("Configuração: apps_per_day=%d, coleta máxima=%d vagas filtradas.", appsPerDay, scraperCollectCap))
	a.pipelineMu.Unlock()

	go func() {
		defer func() {
			// Sincroniza IMAP uma única vez ao final de cada ciclo da pipeline.
			go a.SyncEmailsRoutine()
		}()

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

		if pwErr := ensurePlaywrightRuntime(); pwErr != nil {
			finishWithError("collect", "Playwright indisponível: "+pwErr.Error())
			return
		}

		scraperCmd, err := resolveScraperCommand()
		if err != nil {
			finishWithError("collect", "Não foi possível iniciar coleta: scraper indisponível")
			return
		}
		scraperKeywords := buildScraperSearchKeywords(cfg)
		appsPerDay := cfg.AppsPerDay
		if appsPerDay <= 0 {
			appsPerDay = 50
		}
		scraperCollectCap := appsPerDay * 2

		scraperCmd.Env = mergeCommandEnv(os.Environ(), map[string]string{
			"SEARCH_KEYWORDS":         scraperKeywords,
			"SEARCH_EXCLUDE_KEYWORDS": strings.Join(cfg.SuggestedExcludeKeywords, ","),
			"SEARCH_SENIORITY":        cfg.SuggestedSeniority,
			"SEARCH_REMOTE_POLICY":    cfg.SuggestedRemotePolicy,
			"SEARCH_SOURCES":          strings.Join(cfg.SuggestedSources, ","),
			"SCRAPER_MAX_COLLECT":     strconv.Itoa(scraperCollectCap),
		})
		scraperOutput, scraperErr := runBackgroundCommandWithLogs(scraperCmd, func(line string) {
			a.pipelineMu.Lock()
			a.appendPipelineLogLocked("scraper> " + line)
			a.pipelineMu.Unlock()
		})
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
		readyToApply := a.countReadyApplications()
		if readyToApply <= 0 {
			a.setPipelineStepLocked("apply", "done", "Sem candidaturas FORJADO para envio nesta execução")
			a.pipelineStatus.State = "done"
			a.pipelineStatus.Summary = "Pipeline concluído sem candidaturas elegíveis para envio"
			a.pipelineStatus.FinishedAt = nowISO()
			a.appendPipelineLogLocked("Etapa 4/4: nenhuma candidatura pronta para envio. Pipeline concluído.")
			a.pipelineMu.Unlock()
			return
		}
		a.setPipelineStepLocked("apply", "running", fmt.Sprintf("Executando candidatura automática (%d prontas)", readyToApply))
		a.pipelineStatus.Summary = "Aplicando nas vagas elegíveis..."
		a.appendPipelineLogLocked(fmt.Sprintf("Etapa 4/4: iniciando envio automático de %d candidaturas.", readyToApply))
		a.pipelineMu.Unlock()

		fillerCmd, err := resolveFillerCommand()
		if err != nil {
			finishWithError("apply", "Não foi possível iniciar candidatura: filler indisponível")
			return
		}
		fillerOutput, fillerErr := runBackgroundCommandWithLogs(fillerCmd, func(line string) {
			a.pipelineMu.Lock()
			a.appendPipelineLogLocked("filler> " + line)
			a.pipelineMu.Unlock()
		})
		if fillerErr != nil {
			detail := "Candidatura falhou: " + fillerErr.Error()
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

func mergeCommandEnv(base []string, overrides map[string]string) []string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}

	values := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))
	seen := map[string]struct{}{}

	push := func(key, value string) {
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			order = append(order, key)
		}
		values[key] = value
	}

	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		push(key, value)
	}

	for key, value := range overrides {
		if strings.TrimSpace(key) == "" {
			continue
		}
		push(key, value)
	}

	merged := make([]string, 0, len(order))
	for _, key := range order {
		merged = append(merged, key+"="+values[key])
	}

	return merged
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
	if _, lookErr := exec.LookPath("go"); lookErr == nil {
		if projectRoot, ok := resolveProjectRootForStrict(filepath.Join("cmd", "filler")); ok {
			cmd := exec.Command("go", "run", "./cmd/filler")
			cmd.Dir = projectRoot
			return cmd, nil
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		var candidates []string
		if runtime.GOOS == "windows" {
			candidates = []string{
				filepath.Join(exeDir, "filler.exe"),
				filepath.Join(exeDir, "filler"),
			}
		} else {
			candidates = []string{
				filepath.Join(exeDir, "filler"),
				filepath.Join(exeDir, "filler.exe"),
			}
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

	return nil, fmt.Errorf("filler binary not found and project root for `go run ./cmd/filler` was not detected")
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

func maskedSecretHint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "vazia"
	}
	if len(trimmed) <= 4 {
		return fmt.Sprintf("len=%d final=%s", len(trimmed), strings.Repeat("*", len(trimmed)))
	}
	return fmt.Sprintf("len=%d final=%s", len(trimmed), trimmed[len(trimmed)-4:])
}

func summarizeHTTPBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	preview := strings.TrimSpace(string(body))
	if len(preview) > 240 {
		preview = preview[:240] + "..."
	}
	return preview
}

func classifyProbeError(err error) (string, string) {
	if err == nil {
		return "✗ OFFLINE", "falha desconhecida"
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "✗ TIMEOUT", "timeout de rede ao validar credencial"
		}
		return "✗ OFFLINE", "falha de rede ao validar credencial"
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "no such host"):
		return "✗ DNS", "não foi possível resolver o host da API"
	case strings.Contains(msg, "connection refused"):
		return "✗ CONEXÃO", "conexão recusada pelo destino"
	case strings.Contains(msg, "timeout"):
		return "✗ TIMEOUT", "timeout de rede ao validar credencial"
	default:
		return "✗ OFFLINE", shortenErr(err)
	}
}

func classifyProbeHTTP(service string, statusCode int, body []byte) (string, string) {
	bodyText := strings.ToLower(strings.TrimSpace(string(body)))
	bodyPreview := summarizeHTTPBody(body)

	switch {
	case statusCode >= 200 && statusCode < 300:
		return "✓ OK", "credencial válida e API acessível"
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		if strings.Contains(bodyText, "expired") || strings.Contains(bodyText, "expirada") || strings.Contains(bodyText, "expired key") {
			return "✗ CHAVE EXPIRADA", "a API sinalizou chave expirada"
		}
		return "✗ CHAVE INVÁLIDA", "a API rejeitou autenticação (401/403)"
	case statusCode == http.StatusTooManyRequests:
		detail := "rate limit ativo (429)"
		if bodyPreview != "" {
			detail = detail + ": " + bodyPreview
		}
		return "⚠ RATE LIMIT", detail
	case statusCode >= 500:
		detail := fmt.Sprintf("%s indisponível (HTTP %d)", service, statusCode)
		if bodyPreview != "" {
			detail = detail + ": " + bodyPreview
		}
		return "✗ SERVIÇO", detail
	default:
		detail := fmt.Sprintf("resposta HTTP %d", statusCode)
		if bodyPreview != "" {
			detail = detail + ": " + bodyPreview
		}
		return "✗ ERRO", detail
	}
}

func probeServiceAuth(service string, req *http.Request, client *http.Client) (string, string) {
	resp, err := client.Do(req)
	if err != nil {
		return classifyProbeError(err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if readErr != nil {
		return "✗ ERRO", "falha ao ler resposta da API"
	}
	return classifyProbeHTTP(service, resp.StatusCode, body)
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
	if strings.TrimSpace(os.Getenv("IMAP_USER")) == "" || strings.TrimSpace(os.Getenv("IMAP_PASS")) == "" {
		log.Printf("SyncEmails: IMAP_USER/IMAP_PASS ausentes; sincronização ignorada até configurar credenciais")
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
		classificacao = strings.ToUpper(strings.TrimSpace(classificacao))
		if classificacao == "" {
			classificacao = "OUTRO"
		}

		// Salva a mensagem classificada para a UI conseguir mostrar depois.
		pseudoUUID := uuid.NewString()
		_, execErr := a.database.Exec("INSERT INTO Email_Recrutador (id, email, classificacao, corpo) VALUES (?, ?, ?, ?)",
			pseudoUUID, "recruiter@example.com", classificacao, body)

		if execErr == nil {
			a.reconcileApplicationStatusFromEmail(classificacao, body)
			imapClient.MarkAsSeen(seqId)
			log.Printf("SyncEmails: Appended message [%s]", classificacao)
		}
	}
}

func (a *App) reconcileApplicationStatusFromEmail(classification, body string) {
	if a.database == nil {
		return
	}
	targetStatus := ""
	switch strings.ToUpper(strings.TrimSpace(classification)) {
	case "ENTREVISTA":
		targetStatus = "CONFIRMADA"
	case "REJEICAO":
		targetStatus = "REJEITADA"
	default:
		return
	}

	bodyLower := strings.ToLower(body)
	type candidate struct {
		ID      string
		Title   string
		Company string
		Score   int
	}

	rows, err := a.database.Query(`
		SELECT c.id, COALESCE(v.titulo, ''), COALESCE(v.empresa, '')
		FROM Candidatura_Forjada c
		JOIN Vaga_Prospectada v ON v.id = c.vaga_id
		WHERE c.status IN ('FORJADO', 'ENVIADA', 'APLICADA', 'CONFIRMADA')
	`)
	if err != nil {
		log.Printf("reconcileApplicationStatusFromEmail: query failed: %v", err)
		return
	}
	defer rows.Close()

	best := candidate{}
	found := false
	for rows.Next() {
		var item candidate
		if scanErr := rows.Scan(&item.ID, &item.Title, &item.Company); scanErr != nil {
			continue
		}
		title := strings.ToLower(strings.TrimSpace(item.Title))
		company := strings.ToLower(strings.TrimSpace(item.Company))
		score := 0
		if title != "" && strings.Contains(bodyLower, title) {
			score++
		}
		if company != "" && strings.Contains(bodyLower, company) {
			score += 2
		}
		item.Score = score
		if score <= 0 {
			continue
		}
		if !found || item.Score > best.Score {
			best = item
			found = true
		}
	}

	if !found {
		log.Printf("reconcileApplicationStatusFromEmail: no matching candidatura found for classification %s", classification)
		return
	}

	if _, err := a.database.Exec("UPDATE Candidatura_Forjada SET status = ? WHERE id = ?", targetStatus, best.ID); err != nil {
		log.Printf("reconcileApplicationStatusFromEmail: update failed: %v", err)
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

	body, statusCode, reqErr := doGeminiRequestWithRetry("", apiKey, jsonValue, 4, 1200*time.Millisecond)
	if reqErr != nil {
		return fmt.Sprintf("Falha ao comunicar com Gemini: %v", reqErr)
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Sprintf("Erro da API Gemini: HTTP %d", statusCode)
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

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return fmt.Sprintf("Erro ao parsear resposta: %v", err)
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text
	}

	return "Falha inesperada ao gerar dossiê."
}

// Verifica a saúde das integrações usadas pelo dashboard.
func (a *App) GetSystemStatus() map[string]interface{} {
	envFile, err := godotenv.Read(getAppEnvPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("GetSystemStatus: falha ao ler .env: %v", err)
	}
	readSetting := func(key string) string {
		if v, ok := envFile[key]; ok {
			return strings.TrimSpace(v)
		}
		return strings.TrimSpace(os.Getenv(key))
	}

	status := map[string]interface{}{
		"database":        "✓ OK",
		"database_detail": "",
		"database_path":   a.databasePath,
		"cohere":          "⚠ CHAVE AUSENTE",
		"cohere_detail":   "COHERE_API_KEY não configurada",
		"groq":            "⚠ CHAVE AUSENTE",
		"groq_detail":     "GROQ_API_KEY não configurada",
		"gemini":          "⚠ CHAVE AUSENTE",
		"gemini_detail":   "GEMINI_API_KEY não configurada",
		"imap":            "✗ OFFLINE",
		"imap_detail":     "credenciais IMAP ausentes",
	}

	// Banco local com diagnóstico detalhado para facilitar troubleshooting na sidebar.
	dbState, dbDetail := diagnoseDatabaseStatus(a.database, a.databasePath, a.databaseStartupErr)
	status["database"] = dbState
	status["database_detail"] = dbDetail

	// Cohere
	cohereKey := readSetting("COHERE_API_KEY")
	if cohereKey != "" {
		status["cohere"] = "✗ OFFLINE"
		status["cohere_detail"] = "tentando validar credencial na API Cohere (" + maskedSecretHint(cohereKey) + ")"
		cohere := NewCohereClient()
		cohere.apiKey = cohereKey
		// Faz um teste rápido autenticado na API para validar conectividade.
		req, err := http.NewRequest("GET", "https://api.cohere.ai/v1/models", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+cohere.apiKey)
			req.Header.Set("Accept", "application/json")
			probe, detail := probeServiceAuth("Cohere", req, cohere.client)
			status["cohere"] = probe
			status["cohere_detail"] = detail
		} else {
			status["cohere"] = "✗ ERRO"
			status["cohere_detail"] = "falha ao montar requisição de validação"
		}
	}

	// Groq
	groqKey := readSetting("GROQ_API_KEY")
	if groqKey != "" {
		status["groq"] = "✗ OFFLINE"
		status["groq_detail"] = "tentando validar credencial na API Groq (" + maskedSecretHint(groqKey) + ")"
		req, err := http.NewRequest("GET", "https://api.groq.com/openai/v1/models", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+groqKey)
			probe, detail := probeServiceAuth("Groq", req, outboundHTTPClient)
			status["groq"] = probe
			status["groq_detail"] = detail
		} else {
			status["groq"] = "✗ ERRO"
			status["groq_detail"] = "falha ao montar requisição de validação"
		}
	}

	// Gemini
	geminiKey := readSetting("GEMINI_API_KEY")
	if geminiKey != "" {
		status["gemini"] = "✗ OFFLINE"
		status["gemini_detail"] = "tentando validar credencial na API Gemini (" + maskedSecretHint(geminiKey) + ")"
		probe := []byte(`{"contents":[{"parts":[{"text":"ping"}]}]}`)
		req, err := buildGeminiRequest("", geminiKey, probe)
		if err == nil {
			probeState, detail := probeServiceAuth("Gemini", req, outboundHTTPClient)
			status["gemini"] = probeState
			status["gemini_detail"] = detail
		} else {
			status["gemini"] = "✗ ERRO"
			status["gemini_detail"] = "falha ao montar requisição de validação"
		}
	}

	// IMAP (somente estado de configuração para evitar login em loop no poll da sidebar)
	imapServer := readSetting("IMAP_SERVER")
	imapUser := readSetting("IMAP_USER")
	imapPass := readSetting("IMAP_PASS")
	if imapServer != "" && imapUser != "" && imapPass != "" {
		status["imap"] = "✓ CONFIGURADO"
		status["imap_detail"] = "servidor, usuário e senha informados"
	} else if imapServer != "" || imapUser != "" || imapPass != "" {
		status["imap"] = "⚠ INCOMPLETO"
		status["imap_detail"] = "preencha servidor, usuário e senha para teste IMAP"
	} else {
		status["imap"] = "⚠ CHAVE AUSENTE"
		status["imap_detail"] = "credenciais IMAP não configuradas"
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
	CohereAPIKey     string `json:"cohere_api_key"`
	GroqAPIKey       string `json:"groq_api_key"`
	GeminiAPIKey     string `json:"gemini_api_key"`
	ATSMinScore      string `json:"ats_min_score"`
	ImapServer       string `json:"imap_server"`
	ImapUser         string `json:"imap_user"`
	ImapPass         string `json:"imap_pass"`
	SearchKeywords   string `json:"search_keywords"`
	SearchCountry    string `json:"search_country"`
	GupyBoards       string `json:"gupy_company_urls"`
	GreenhouseBoards string `json:"greenhouse_boards"`
	LeverCompanies   string `json:"lever_companies"`
}

type ProfileDTO struct {
	TargetRoles              []string `json:"target_roles"`
	CoreStack                []string `json:"core_stack"`
	SuggestedKeywords        []string `json:"suggested_keywords"`
	SuggestedExcludeKeywords []string `json:"suggested_exclude_keywords"`
	SuggestedSeniority       string   `json:"suggested_seniority"`
	SuggestedRemotePolicy    string   `json:"suggested_remote_policy"`
	SuggestedSources         []string `json:"suggested_sources"`
	GeminiRationale          string   `json:"gemini_rationale"`
	StrictlyRemote           bool     `json:"strictly_remote"`
	MinSalaryFloor           string   `json:"min_salary_floor"`
	AppsPerDay               int      `json:"apps_per_day"`
	SourceFile               string   `json:"source_file"`
	ParseStatus              string   `json:"parse_status"`
	ParseErrorMessage        string   `json:"parse_error_message"`
}

type sessionCookieDTO struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// Carrega o mapeamento local do .env para a tela de configurações do frontend.
func (a *App) LoadSettings() SettingsDTO {
	_ = godotenv.Load(getAppEnvPath())
	return SettingsDTO{
		// Não devolve segredos para o frontend por padrão.
		CohereAPIKey:     "",
		GroqAPIKey:       "",
		GeminiAPIKey:     "",
		ATSMinScore:      envOrDefault("ATS_MIN_SCORE", "0.40"),
		ImapServer:       os.Getenv("IMAP_SERVER"),
		ImapUser:         os.Getenv("IMAP_USER"),
		ImapPass:         "",
		SearchKeywords:   envOrDefault("SEARCH_KEYWORDS", "software engineer,backend,java,golang"),
		SearchCountry:    envOrDefault("SEARCH_COUNTRY", "BR"),
		GupyBoards:       os.Getenv("GUPY_COMPANY_URLS"),
		GreenhouseBoards: os.Getenv("GREENHOUSE_BOARDS"),
		LeverCompanies:   os.Getenv("LEVER_COMPANIES"),
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

	// Atualiza o ambiente em runtime para que status checks e workers
	// usem imediatamente as credenciais recém-salvas.
	applyRuntimeSettingsEnv(envMap)

	return true
}

// ClearPersistedSecrets limpa segredos salvos no .env local para facilitar rotação de chaves.
func (a *App) ClearPersistedSecrets() string {
	envPath := getAppEnvPath()
	existing, err := godotenv.Read(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("ClearPersistedSecrets: falha ao ler .env atual: %v", err)
		return "Falha ao ler configurações persistidas."
	}
	if existing == nil {
		existing = map[string]string{}
	}

	for _, key := range []string{"COHERE_API_KEY", "GROQ_API_KEY", "GEMINI_API_KEY", "IMAP_PASS"} {
		existing[key] = ""
	}

	if writeErr := godotenv.Write(existing, envPath); writeErr != nil {
		log.Printf("ClearPersistedSecrets: falha ao escrever .env: %v", writeErr)
		return "Falha ao limpar segredos persistidos."
	}
	if chmodErr := os.Chmod(envPath, 0o600); chmodErr != nil {
		log.Printf("ClearPersistedSecrets: aviso ao aplicar permissão restrita no .env: %v", chmodErr)
	}

	_ = os.Setenv("COHERE_API_KEY", "")
	_ = os.Setenv("GROQ_API_KEY", "")
	_ = os.Setenv("GEMINI_API_KEY", "")
	_ = os.Setenv("IMAP_PASS", "")

	return "Segredos limpos com sucesso."
}

func resolveSecretEnvKey(raw string) (string, string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	switch key {
	case "cohere", "cohere_api_key":
		return "COHERE_API_KEY", "Cohere", true
	case "groq", "groq_api_key":
		return "GROQ_API_KEY", "Groq", true
	case "gemini", "gemini_api_key":
		return "GEMINI_API_KEY", "Gemini", true
	case "imap", "imap_pass", "imap_password":
		return "IMAP_PASS", "IMAP Password", true
	default:
		return "", "", false
	}
}

// ClearPersistedSecret limpa apenas um segredo persistido específico.
func (a *App) ClearPersistedSecret(secret string) string {
	envKey, label, ok := resolveSecretEnvKey(secret)
	if !ok {
		return "Segredo não suportado para limpeza."
	}

	envPath := getAppEnvPath()
	existing, err := godotenv.Read(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("ClearPersistedSecret: falha ao ler .env atual: %v", err)
		return "Falha ao ler configurações persistidas."
	}
	if existing == nil {
		existing = map[string]string{}
	}

	existing[envKey] = ""

	if writeErr := godotenv.Write(existing, envPath); writeErr != nil {
		log.Printf("ClearPersistedSecret: falha ao escrever .env: %v", writeErr)
		return "Falha ao limpar segredo persistido."
	}
	if chmodErr := os.Chmod(envPath, 0o600); chmodErr != nil {
		log.Printf("ClearPersistedSecret: aviso ao aplicar permissão restrita no .env: %v", chmodErr)
	}

	_ = os.Setenv(envKey, "")
	return label + " limpo com sucesso."
}

// VerifySingleCredential valida somente uma credencial de IA sem impactar as demais.
func (a *App) VerifySingleCredential(service, apiKey string) map[string]string {
	out := map[string]string{
		"service": strings.ToLower(strings.TrimSpace(service)),
		"status":  "✗ ERRO",
		"detail":  "serviço não suportado",
	}

	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		out["status"] = "⚠ CHAVE AUSENTE"
		out["detail"] = "nenhuma chave informada para validação"
		return out
	}

	switch out["service"] {
	case "cohere":
		req, err := http.NewRequest("GET", "https://api.cohere.ai/v1/models", nil)
		if err != nil {
			out["detail"] = "falha ao montar requisição de validação"
			return out
		}
		req.Header.Set("Authorization", "Bearer "+trimmed)
		req.Header.Set("Accept", "application/json")
		state, detail := probeServiceAuth("Cohere", req, outboundHTTPClient)
		out["status"] = state
		out["detail"] = detail
		return out

	case "groq":
		req, err := http.NewRequest("GET", "https://api.groq.com/openai/v1/models", nil)
		if err != nil {
			out["detail"] = "falha ao montar requisição de validação"
			return out
		}
		req.Header.Set("Authorization", "Bearer "+trimmed)
		state, detail := probeServiceAuth("Groq", req, outboundHTTPClient)
		out["status"] = state
		out["detail"] = detail
		return out

	case "gemini":
		probe := []byte(`{"contents":[{"parts":[{"text":"ping"}]}]}`)
		req, err := buildGeminiRequest("", trimmed, probe)
		if err != nil {
			out["detail"] = "falha ao montar requisição de validação"
			return out
		}
		state, detail := probeServiceAuth("Gemini", req, outboundHTTPClient)
		out["status"] = state
		out["detail"] = detail
		return out

	default:
		return out
	}
}

func applyRuntimeSettingsEnv(envMap map[string]string) {
	if envMap == nil {
		return
	}
	if v, ok := envMap["COHERE_API_KEY"]; ok {
		_ = os.Setenv("COHERE_API_KEY", strings.TrimSpace(v))
	}
	if v, ok := envMap["GROQ_API_KEY"]; ok {
		_ = os.Setenv("GROQ_API_KEY", strings.TrimSpace(v))
	}
	if v, ok := envMap["GEMINI_API_KEY"]; ok {
		_ = os.Setenv("GEMINI_API_KEY", strings.TrimSpace(v))
	}
	if v, ok := envMap["IMAP_SERVER"]; ok {
		_ = os.Setenv("IMAP_SERVER", strings.TrimSpace(v))
	}
	if v, ok := envMap["IMAP_USER"]; ok {
		_ = os.Setenv("IMAP_USER", strings.TrimSpace(v))
	}
	if v, ok := envMap["IMAP_PASS"]; ok {
		_ = os.Setenv("IMAP_PASS", strings.TrimSpace(v))
	}
	if v, ok := envMap["ATS_MIN_SCORE"]; ok {
		_ = os.Setenv("ATS_MIN_SCORE", strings.TrimSpace(v))
	}
	if v, ok := envMap["SEARCH_KEYWORDS"]; ok {
		_ = os.Setenv("SEARCH_KEYWORDS", strings.TrimSpace(v))
	}
	if v, ok := envMap["SEARCH_COUNTRY"]; ok {
		_ = os.Setenv("SEARCH_COUNTRY", strings.TrimSpace(v))
	}
	if v, ok := envMap["GUPY_COMPANY_URLS"]; ok {
		_ = os.Setenv("GUPY_COMPANY_URLS", strings.TrimSpace(v))
	}
	if v, ok := envMap["GREENHOUSE_BOARDS"]; ok {
		_ = os.Setenv("GREENHOUSE_BOARDS", strings.TrimSpace(v))
	}
	if v, ok := envMap["LEVER_COMPANIES"]; ok {
		_ = os.Setenv("LEVER_COMPANIES", strings.TrimSpace(v))
	}
}

// Abre o seletor nativo, extrai o texto do PDF e pede ao Gemini o JSON estruturado.
func (a *App) UploadAndParseCV() ProfileDTO {
	baseProfile := ProfileDTO{
		StrictlyRemote:           true,
		MinSalaryFloor:           "$120,000",
		AppsPerDay:               50,
		SuggestedKeywords:        []string{},
		SuggestedExcludeKeywords: []string{},
		SuggestedSeniority:       "any",
		SuggestedRemotePolicy:    "strict-remote",
		SuggestedSources:         []string{},
		GeminiRationale:          "",
		ParseStatus:              "idle",
		ParseErrorMessage:        "",
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
		baseProfile.ParseErrorMessage = "Upload cancelado pelo usuário."
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
		baseProfile.ParseErrorMessage = "Não foi possível abrir o PDF selecionado."
		return baseProfile
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		log.Printf("UploadAndParseCV: Fail to extract text: %v\n", err)
		baseProfile.ParseStatus = "error"
		baseProfile.ParseErrorMessage = "Falha ao extrair texto do PDF."
		return baseProfile
	}
	buf.ReadFrom(b)
	textContent := buf.String()
	if strings.TrimSpace(textContent) == "" {
		baseProfile.ParseStatus = "error"
		baseProfile.ParseErrorMessage = "O PDF não contém texto legível para análise."
		return baseProfile
	}

	// 3. Monta a requisição para o Gemini estruturar a estratégia de busca.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Println("UploadAndParseCV: No GEMINI_API_KEY found")
		baseProfile.ParseStatus = "uploaded"
		baseProfile.ParseErrorMessage = "GEMINI_API_KEY não configurada."
		return baseProfile
	}

	prompt := `I am providing a CV in raw text below.
Extract practical search guidance for job prospecting from this CV.
Return ONLY one valid JSON object starting with '{' and ending with '}' using this exact schema:
{
  "target_roles": ["..."],
  "core_stack": ["..."],
  "suggested_keywords": ["..."],
	"suggested_exclude_keywords": ["..."],
	"suggested_seniority": "any|junior|mid|senior|staff|lead",
	"suggested_remote_policy": "any|remote-first|strict-remote",
	"suggested_sources": ["linkedin|gupy|greenhouse|lever"],
  "gemini_rationale": "short explanation"
}
Rules:
- max 12 target_roles
- max 12 core_stack
- max 18 suggested_keywords
- max 12 suggested_exclude_keywords
- prefer strict-remote when CV is clearly remote-oriented
- keep each item concise
- no markdown or code fences
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
	body, statusCode, reqErr := doGeminiRequestWithRetry("", apiKey, jsonValue, 4, 1200*time.Millisecond)
	if reqErr != nil {
		log.Printf("UploadAndParseCV: Gemini call failed after retry: %v", reqErr)
		baseProfile.ParseStatus = "error"
		baseProfile.ParseErrorMessage = "Falha de comunicação com a API Gemini."
		return baseProfile
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Printf("UploadAndParseCV: Gemini returned HTTP %d body=%s", statusCode, strings.TrimSpace(string(body)))
		baseProfile.ParseStatus = "error"
		if statusCode == http.StatusTooManyRequests {
			baseProfile.ParseErrorMessage = "Gemini em rate limit (HTTP 429). Aguarde alguns segundos e tente novamente."
		} else {
			baseProfile.ParseErrorMessage = fmt.Sprintf("Gemini retornou HTTP %d.", statusCode)
		}
		return baseProfile
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

	if decErr := json.NewDecoder(bytes.NewReader(body)).Decode(&result); decErr != nil {
		log.Printf("UploadAndParseCV: Failed to decode Gemini Response: %v", decErr)
		baseProfile.ParseStatus = "error"
		baseProfile.ParseErrorMessage = "Falha ao decodificar resposta do Gemini."
		return baseProfile
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		rawGemini := string(bytes.TrimSpace([]byte(result.Candidates[0].Content.Parts[0].Text)))
		geminiJSON, extractErr := extractJSONObjectFromText(rawGemini)
		if extractErr != nil {
			log.Printf("UploadAndParseCV: Could not extract JSON from Gemini response: %v", extractErr)
			baseProfile.ParseStatus = "error"
			baseProfile.ParseErrorMessage = "Resposta do Gemini não veio em JSON válido."
			return baseProfile
		}
		log.Printf("UploadAndParseCV: Gemini JSON recebido (%d bytes)", len(geminiJSON))
		var parsed ProfileDTO
		parsed = baseProfile

		if err := json.Unmarshal([]byte(geminiJSON), &parsed); err != nil {
			log.Printf("UploadAndParseCV: Could not unmarshal string format: %v", err)
			baseProfile.ParseStatus = "error"
			baseProfile.ParseErrorMessage = "Não foi possível interpretar o JSON retornado pelo Gemini."
			return baseProfile
		}

		parsed.TargetRoles = sanitizeGeminiTerms(parsed.TargetRoles, 12)
		parsed.CoreStack = sanitizeGeminiTerms(parsed.CoreStack, 12)
		parsed.SuggestedKeywords = sanitizeGeminiTerms(parsed.SuggestedKeywords, 18)
		parsed.SuggestedExcludeKeywords = sanitizeGeminiTerms(parsed.SuggestedExcludeKeywords, 12)
		parsed.SuggestedSeniority = sanitizeGeminiEnum(parsed.SuggestedSeniority, map[string]struct{}{
			"any": {}, "junior": {}, "mid": {}, "senior": {}, "staff": {}, "lead": {},
		}, "any")
		parsed.SuggestedRemotePolicy = sanitizeGeminiEnum(parsed.SuggestedRemotePolicy, map[string]struct{}{
			"any": {}, "remote-first": {}, "strict-remote": {},
		}, "strict-remote")
		parsed.SuggestedSources = sanitizeGeminiSources(parsed.SuggestedSources)
		parsed.GeminiRationale = sanitizeGeminiRationale(parsed.GeminiRationale, 360)

		parsed.SourceFile = baseProfile.SourceFile
		parsed.ParseStatus = "parsed"
		parsed.ParseErrorMessage = ""

		return parsed
	}

	log.Printf("UploadAndParseCV: Gemini retornou sem candidates")
	baseProfile.ParseStatus = "error"
	baseProfile.ParseErrorMessage = "Gemini não retornou conteúdo candidato para parse."
	return baseProfile
}

func (a *App) ConnectLinkedInSession() string {
	if err := ensurePlaywrightRuntime(); err != nil {
		return "Falha ao preparar Playwright: " + err.Error()
	}

	pw, err := playwright.Run()
	if err != nil {
		return "Falha ao iniciar Playwright: " + err.Error()
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-infobars",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-dev-shm-usage",
			"--window-size=1366,900",
		},
	})
	if err != nil {
		return "Falha ao abrir navegador para login LinkedIn: " + err.Error()
	}
	defer browser.Close()

	ctx, err := browser.NewContext()
	if err != nil {
		return "Falha ao criar contexto de sessão: " + err.Error()
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		return "Falha ao abrir página de login LinkedIn: " + err.Error()
	}
	defer page.Close()

	if _, err := page.Goto("https://www.linkedin.com/login", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(45000),
	}); err != nil {
		return "Falha ao navegar para login LinkedIn: " + err.Error()
	}

	deadline := time.Now().Add(5 * time.Minute)
	loggedIn := false
	for time.Now().Before(deadline) {
		current := strings.ToLower(page.URL())
		if strings.Contains(current, "linkedin.com/feed") || strings.Contains(current, "linkedin.com/jobs") || strings.Contains(current, "linkedin.com/mynetwork") {
			loggedIn = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !loggedIn {
		return "Tempo esgotado aguardando login no LinkedIn. Tente novamente e conclua o login no navegador aberto."
	}

	cookies, err := ctx.Cookies()
	if err != nil {
		return "Falha ao extrair cookies da sessão: " + err.Error()
	}
	if len(cookies) == 0 {
		return "Sessão concluída, mas nenhum cookie foi capturado."
	}

	sessionCookies := make([]sessionCookieDTO, 0, len(cookies))
	for _, c := range cookies {
		sameSite := ""
		if c.SameSite != nil {
			sameSite = string(*c.SameSite)
		}
		sessionCookies = append(sessionCookies, sessionCookieDTO{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
			SameSite: sameSite,
		})
	}

	appDir := GetAppDir()
	sessionPath := strings.TrimSpace(os.Getenv("SESSION_PATH"))
	if sessionPath == "" {
		sessionPath = "session.json"
	}
	if !filepath.IsAbs(sessionPath) {
		sessionPath = filepath.Join(appDir, sessionPath)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		return "Falha ao preparar diretório da sessão: " + err.Error()
	}

	payload, err := json.MarshalIndent(sessionCookies, "", "  ")
	if err != nil {
		return "Falha ao serializar cookies da sessão: " + err.Error()
	}
	if err := os.WriteFile(sessionPath, payload, 0o600); err != nil {
		return "Falha ao salvar session.json: " + err.Error()
	}
	_ = os.Setenv("SESSION_PATH", sessionPath)

	return "Sessão LinkedIn conectada com sucesso e salva em " + sessionPath
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
