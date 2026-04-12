package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

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
	// Tarefa 39: inicializa a configuração.
	_ = godotenv.Load()

	dbPath := mustEnv("DATABASE_URL")
	dbKey := mustEnv("DB_ENCRYPTION_KEY")
	groqKey := mustEnv("GROQ_API_KEY")
	sessionPath := getEnv("SESSION_PATH", "session.json")

	// Pré-carrega o cliente Groq com a chave da sessão.
	groqClient := llm.NewGroqClient(groqKey)

	// Etapa 2: abre o SQLite criptografado.
	database, err := db.Open(dbPath, dbKey)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	candidaturas, err := loadForjadoTargets(database)
	if err != nil {
		return fmt.Errorf("failed to load candidates: %w", err)
	}

	if len(candidaturas) == 0 {
		log.Println("filler: no FORJADO applications found. Exiting.")
		return nil
	}

	log.Printf("filler: found %d applications ready for submission", len(candidaturas))

	// Etapa 3: inicializa o Playwright.
	p, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to run playwright: %w", err)
	}
	defer p.Stop()

	// Carrega os contextos existentes com cookies antes de iniciar o loop.
	browser, err := pw.NewBrowser(p)
	if err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("failed to create browser context: %w", err)
	}
	defer ctx.Close()

	if err := pw.LoadCookies(ctx, sessionPath); err != nil {
		log.Printf("filler: warning, unable to load cookies: %v", err)
	}

	// Itera pelas candidaturas carregadas e executa o fluxo de automação.
	for _, c := range candidaturas {
		err := processApplication(ctx, groqClient, c)
		if err != nil {
			log.Printf("filler: error processing application %s: %v", c.Candidatura.ID, err)
			updateStatus(database, c.Candidatura.ID, domain.StatusErro)
		} else {
			log.Printf("filler: application %s successfully sent!", c.Candidatura.ID)
			updateStatus(database, c.Candidatura.ID, domain.StatusAplicada)
		}
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
		log.Printf("filler: critical error updating DB status to %s for %s: %v", status, candidaturaID, err)
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return val
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
