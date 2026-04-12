// Command scraper é o coletor principal do GhostApply.
//
// Pipeline:
//  1. Carrega configuração do .env
//  2. Abre o banco SQLite criptografado
//  3. Sobe o Playwright com endurecimento contra bot
//  4. Injeta cookies da sessão salva
//  5. Navega até a busca de vagas remotas
//  6. Extrai os cards de vaga
//  7. Sanitiza descrições para reduzir risco de prompt injection
//  8. Persiste no banco com status PENDENTE
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
	// ── Etapa 1: carrega a configuração do ambiente ─────────────────────────
	if err := godotenv.Load(); err != nil {
		log.Println("scraper: nenhum .env encontrado, usando o ambiente do sistema")
	}

	dbPath := mustEnv("DATABASE_URL")
	dbKey  := mustEnv("DB_ENCRYPTION_KEY")
	sessionPath := getEnv("SESSION_PATH", "session.json")
	keywords    := getEnv("SEARCH_KEYWORDS", "golang engineer")

	// ── Etapa 2: abre o banco criptografado ─────────────────────────────────
	database, err := db.Open(dbPath, dbKey)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	defer database.Close()

	// ── Etapas 3 e 4: configura o Playwright ─────────────────────────────────
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
		return fmt.Errorf("run: falha ao criar contexto do navegador: %w", err)
	}
	defer ctx.Close()

	// Injeta os cookies da sessão antes de qualquer navegação.
	if err := LoadCookies(ctx, sessionPath); err != nil {
		log.Printf("scraper: warning — could not load cookies from %s: %v", sessionPath, err)
		log.Println("scraper: continuing without session (expect login wall)")
	}

	page, err := ctx.NewPage()
	if err != nil {
		return fmt.Errorf("run: failed to open page: %w", err)
	}
	defer page.Close()

	// ── Etapa 5: navega até a busca remota do LinkedIn ──────────────────────
	if err := NavigateToLinkedInSearch(page, keywords); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// ── Etapa 6: extrai os cards de vaga ─────────────────────────────────────
	vagas, err := ExtractVagas(page)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	log.Printf("scraper: extracted %d job cards", len(vagas))

	// ── Etapas 7 e 8: sanitiza e persiste ───────────────────────────────────
	saved := 0
	for i := range vagas {
		vagas[i].ID = uuid.NewString()

		// Remove scripts, emails e URLs da descrição bruta.
		if vagas[i].Descricao != "" {
			clean, err := parser.Clean(vagas[i].Descricao)
			if err != nil {
				log.Printf("scraper: ignorando vaga %s — descrição vazia após limpeza", vagas[i].ID)
				continue
			}
			vagas[i].Descricao = clean
		}

		// Persiste com status PENDENTE.
		if err := insertVaga(database, vagas[i]); err != nil {
			log.Printf("scraper: falha ao salvar vaga %s: %v", vagas[i].ID, err)
			continue
		}
		saved++
	}

	log.Printf("scraper: salvou %d/%d vagas no banco", saved, len(vagas))
	return nil
}

// insertVaga persiste uma Vaga usando INSERT OR IGNORE, mantendo idempotência por URL.
// O status inicial fica como PENDENTE para o processamento seguinte.
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

// mustEnv retorna uma variável de ambiente ou encerra com mensagem clara.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("scraper: required environment variable %s is not set", key)
	}
	return v
}

// getEnv retorna uma variável de ambiente ou um valor padrão.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
