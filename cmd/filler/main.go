package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Wanjos-eng/GhostApply/internal/db"
	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/Wanjos-eng/GhostApply/internal/infra/llm"
	"github.com/Wanjos-eng/GhostApply/internal/infra/pw"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("filler: fatal error: %v", err)
	}
}

func run() error {
	// Inicializa a configuração de ambiente.
	if err := godotenv.Load(); err != nil {
		if appDir, ok := loadRuntimeEnv(); ok {
			log.Printf("filler: loaded runtime .env from %s", appDir)
		}
	}

	appDir := runtimeAppDir()

	dbPath := getEnv("DATABASE_URL", "")
	if dbPath == "" {
		if strings.TrimSpace(appDir) != "" {
			dbPath = filepath.Join(appDir, "forja_ghost.sqlite")
		} else {
			log.Fatalf("filler: DATABASE_URL ausente e não encontrou UserHomeDir")
		}
	}
	dbPath = normalizeRuntimeDataPath(dbPath, appDir)
	dbKey := getEnv("DB_ENCRYPTION_KEY", "")
	if strings.TrimSpace(dbKey) == "" {
		log.Printf("filler: DB_ENCRYPTION_KEY vazio; tentando modo SQLite sem chave")
	}
	groqKey := mustEnv("GROQ_API_KEY")
	sessionPath := getEnv("SESSION_PATH", "session.json")
	if !filepath.IsAbs(sessionPath) && strings.TrimSpace(appDir) != "" {
		sessionPath = filepath.Join(appDir, sessionPath)
	}

	// Pré-carrega o cliente Groq com a chave da sessão.
	groqClient := llm.NewGroqClient(groqKey)

	// Etapa 2: abre o SQLite criptografado.
	database, err := db.Open(dbPath, dbKey)
	if err != nil {
		return fmt.Errorf("falha ao abrir o banco: %w", err)
	}
	defer database.Close()

	candidaturas, err := loadForjadoTargets(database)
	if err != nil {
		return fmt.Errorf("falha ao carregar candidaturas: %w", err)
	}

	if len(candidaturas) == 0 {
		log.Println("filler: nenhuma candidatura FORJADO encontrada; encerrando")
		return nil
	}

	log.Printf("filler: %d candidaturas prontas para envio", len(candidaturas))

	// Etapa 3: inicializa o Playwright.
	p, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("falha ao iniciar o playwright: %w", err)
	}
	defer p.Stop()

	// Carrega os contextos existentes com cookies antes de iniciar o loop.
	browser, err := pw.NewBrowser(p)
	if err != nil {
		return fmt.Errorf("falha ao iniciar o navegador: %w", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("falha ao criar o contexto do navegador: %w", err)
	}
	defer ctx.Close()

	if err := pw.LoadCookies(ctx, sessionPath); err != nil {
		log.Printf("filler: aviso, não foi possível carregar cookies: %v", err)
	}

	// Itera pelas candidaturas carregadas e executa o fluxo de automação.
	successCount := 0
	failureCount := 0
	for _, c := range candidaturas {
		err := processApplication(ctx, groqClient, c)
		if err != nil {
			log.Printf("filler: erro ao processar candidatura %s: %v", c.Candidatura.ID, err)
			updateStatus(database, c.Candidatura.ID, domain.StatusErro)
			failureCount++
		} else {
			log.Printf("filler: candidatura %s enviada com sucesso", c.Candidatura.ID)
			updateStatus(database, c.Candidatura.ID, domain.StatusAplicada)
			successCount++
		}
	}

	log.Printf("filler: resumo de envio => sucesso=%d falhas=%d total=%d", successCount, failureCount, len(candidaturas))
	if failureCount > 0 {
		return fmt.Errorf("envio concluído com falhas: sucesso=%d falhas=%d total=%d", successCount, failureCount, len(candidaturas))
	}

	return nil
}

func loadForjadoTargets(database *sql.DB) ([]domain.VagaComCandidatura, error) {
	rows, err := database.Query(`
		SELECT v.id, v.url, c.id, c.curriculo_path 
		FROM Vaga_Prospectada v
		JOIN Candidatura_Forjada c ON v.id = c.vaga_id
		WHERE c.status = 'FORJADO' -- Fetching exclusively FORJADO 
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.VagaComCandidatura
	for rows.Next() {
		var vc domain.VagaComCandidatura
		if err := rows.Scan(&vc.Vaga.ID, &vc.Vaga.URL, &vc.Candidatura.ID, &vc.Candidatura.CurriculoPath); err != nil {
			return nil, err
		}
		results = append(results, vc)
	}
	return results, nil
}

func updateStatus(database *sql.DB, candidaturaID string, status domain.Status) {
	_, err := database.Exec("UPDATE Candidatura_Forjada SET status = ? WHERE id = ?", string(status), candidaturaID)
	if err != nil {
		log.Printf("filler: erro crítico ao atualizar status %s para %s: %v", status, candidaturaID, err)
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("variável de ambiente obrigatória ausente: %s", key)
	}
	return val
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func runtimeAppDir() string {
	if cfgDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(cfgDir) != "" {
		return filepath.Join(cfgDir, "GhostApply")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".ghostapply")
	}
	return ""
}

func loadRuntimeEnv() (string, bool) {
	appDir := runtimeAppDir()
	if strings.TrimSpace(appDir) != "" {
		envPath := filepath.Join(appDir, ".env")
		if err := godotenv.Load(envPath); err == nil {
			return appDir, true
		}
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		legacyDir := filepath.Join(home, ".ghostapply")
		legacyEnvPath := filepath.Join(legacyDir, ".env")
		if err := godotenv.Load(legacyEnvPath); err == nil {
			return legacyDir, true
		}
	}

	return "", false
}

func normalizeRuntimeDataPath(rawPath, appDir string) string {
	trimmed := strings.Trim(strings.TrimSpace(rawPath), "\"'")
	if trimmed == "" {
		return rawPath
	}
	if trimmed == ":memory:" {
		return trimmed
	}

	legacy := trimmed
	if strings.HasPrefix(strings.ToLower(legacy), "file:") {
		legacy = legacy[len("file:"):]
		legacy = strings.TrimPrefix(legacy, "//")
	}
	if idx := strings.Index(legacy, "?"); idx >= 0 {
		legacy = legacy[:idx]
	}
	if strings.TrimSpace(legacy) == "" {
		legacy = "forja_ghost.sqlite"
	}

	isWindowsAbs := runtime.GOOS == "windows" && len(legacy) >= 3 && (legacy[1] == ':' && (legacy[2] == '\\' || legacy[2] == '/'))
	if !filepath.IsAbs(legacy) && !isWindowsAbs && strings.TrimSpace(appDir) != "" {
		legacy = filepath.Join(appDir, legacy)
	}

	clean := filepath.Clean(legacy)
	if resolved, err := filepath.Abs(clean); err == nil {
		return resolved
	}
	return clean
}
