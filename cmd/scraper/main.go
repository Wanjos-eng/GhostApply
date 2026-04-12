// Command scraper is the GhostApply stealth scraper.
//
// Pipeline:
//  1. Load config from .env
//  2. Open AES-256 encrypted database
//  3. Launch Playwright (headless + anti-bot)
//  4. Inject session cookies from session.json
//  5. Navigate to LinkedIn remote job search
//  6. Extract job cards concurrently
//  7. Sanitise descriptions (SecOps: prompt injection prevention)
//  8. Persist to encrypted database with status PENDENTE
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/Wanjos-eng/GhostApply/internal/db"
	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/Wanjos-eng/GhostApply/internal/parser"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("scraper: fatal error: %v", err)
	}
}

func run() error {
	// ── Step 1: Load environment config ──────────────────────────────────────
	if err := godotenv.Load(); err != nil {
		log.Println("scraper: no .env file found, using system environment")
	}

	dbPath := mustEnv("DATABASE_URL")
	dbKey  := mustEnv("DB_ENCRYPTION_KEY")
	sessionPath := getEnv("SESSION_PATH", "session.json")
	keywords    := getEnv("SEARCH_KEYWORDS", "golang engineer")

	// ── Step 2: Open encrypted database ──────────────────────────────────────
	database, err := db.Open(dbPath, dbKey)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	defer database.Close()

	// ── Step 3 & 4: Playwright setup ──────────────────────────────────────────
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("run: failed to start Playwright: %w", err)
	}
	defer pw.Stop()

	browser, err := NewBrowser(pw)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("run: failed to create browser context: %w", err)
	}
	defer ctx.Close()

	// Task 17: inject session cookies before any navigation
	if err := LoadCookies(ctx, sessionPath); err != nil {
		log.Printf("scraper: warning — could not load cookies from %s: %v", sessionPath, err)
		log.Println("scraper: continuing without session (expect login wall)")
	}

	page, err := ctx.NewPage()
	if err != nil {
		return fmt.Errorf("run: failed to open page: %w", err)
	}
	defer page.Close()

	// ── Step 5: Navigate to LinkedIn remote search ────────────────────────────
	if err := NavigateToLinkedInSearch(page, keywords); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// ── Step 6: Extract job cards concurrently ────────────────────────────────
	vagas, err := ExtractVagas(page)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	log.Printf("scraper: extracted %d job cards", len(vagas))

	// ── Steps 7 & 8: Sanitise + persist ──────────────────────────────────────
	saved := 0
	for i := range vagas {
		vagas[i].ID = uuid.NewString()

		// Task 24 (SecOps): strip scripts, emails, URLs from raw description
		if vagas[i].Descricao != "" {
			clean, err := parser.Clean(vagas[i].Descricao)
			if err != nil {
				log.Printf("scraper: skipping vaga %s — empty description after clean", vagas[i].ID)
				continue
			}
			vagas[i].Descricao = clean
		}

		// Task 25: persist with status PENDENTE
		if err := insertVaga(database, vagas[i]); err != nil {
			log.Printf("scraper: failed to save vaga %s: %v", vagas[i].ID, err)
			continue
		}
		saved++
	}

	log.Printf("scraper: saved %d/%d vagas to database", saved, len(vagas))
	return nil
}

// insertVaga persists a Vaga using INSERT OR IGNORE (idempotent on URL).
// Task 25: status is set to PENDENTE for downstream AI processing.
func insertVaga(database *sql.DB, v domain.Vaga) error {
	_, err := database.Exec(
		`INSERT OR IGNORE INTO Vaga_Prospectada (id, titulo, empresa, url, descricao, status, recrutador_nome, recrutador_perfil)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Titulo, v.Empresa, v.URL, v.Descricao, string(domain.StatusPendente), v.RecrutadorNome, v.RecrutadorPerfil,
	)
	if err != nil {
		return fmt.Errorf("insertVaga: %w", err)
	}
	return nil
}

// mustEnv returns the value of an environment variable or exits with a clear message.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("scraper: required environment variable %s is not set", key)
	}
	return v
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
