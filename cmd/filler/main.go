package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
		if home, errHome := os.UserHomeDir(); errHome == nil {
			godotenv.Load(filepath.Join(home, ".ghostapply", ".env"))
		}
	}

	dbPath := getEnv("DATABASE_URL", "")
	if dbPath == "" {
		if home, errHome := os.UserHomeDir(); errHome == nil {
			dbPath = filepath.Join(home, ".ghostapply", "forja_ghost.sqlite")
		} else {
			log.Fatalf("filler: DATABASE_URL ausente e não encontrou UserHomeDir")
		}
	}
	dbKey := mustEnv("DB_ENCRYPTION_KEY")
	groqKey := mustEnv("GROQ_API_KEY")
	sessionPath := getEnv("SESSION_PATH", "session.json")

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
	for _, c := range candidaturas {
		err := processApplication(ctx, groqClient, c)
		if err != nil {
			log.Printf("filler: erro ao processar candidatura %s: %v", c.Candidatura.ID, err)
			updateStatus(database, c.Candidatura.ID, domain.StatusErro)
		} else {
			log.Printf("filler: candidatura %s enviada com sucesso", c.Candidatura.ID)
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
