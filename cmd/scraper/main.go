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
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Wanjos-eng/GhostApply/internal/db"
	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/Wanjos-eng/GhostApply/internal/parser"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"
)

type searchStrategy struct {
	excludeKeywords map[string]struct{}
	seniority       string
	remotePolicy    string
	allowedSources  map[string]struct{}
}

type strategyFilterStats struct {
	InputCount         int
	KeptCount          int
	DroppedBySource    int
	DroppedByExclude   int
	DroppedBySeniority int
	DroppedByRemote    int
}

type vagaRank struct {
	vaga        domain.Vaga
	titleScore  int
	keywordHits int
	remoteBoost int
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("scraper: fatal error: %v", err)
	}
}

func run() error {
	// ── Etapa 1: carrega a configuração do ambiente ─────────────────────────
	// Tenta carregar .env local, e como fallback usa o master-folder de deploy
	if err := godotenv.Load(); err != nil {
		if appDir, ok := loadRuntimeEnv(); ok {
			log.Printf("scraper: loaded runtime .env from %s", appDir)
		} else {
			log.Println("scraper: nenhum .env encontrado, usando o ambiente do sistema")
		}
	}

	appDir := runtimeAppDir()

	dbPath := getEnv("DATABASE_URL", "")
	if dbPath == "" {
		if strings.TrimSpace(appDir) != "" {
			dbPath = filepath.Join(appDir, "forja_ghost.sqlite")
		} else {
			log.Fatalf("scraper: DATABASE_URL ausente e não encontrou UserHomeDir")
		}
	}
	dbPath = normalizeRuntimeDataPath(dbPath, appDir)
	dbKey := getEnv("DB_ENCRYPTION_KEY", "")
	if strings.TrimSpace(dbKey) == "" {
		log.Printf("scraper: DB_ENCRYPTION_KEY vazio; tentando modo SQLite sem chave")
	}
	sessionPath := getEnv("SESSION_PATH", "session.json")
	if !filepath.IsAbs(sessionPath) && strings.TrimSpace(appDir) != "" {
		sessionPath = filepath.Join(appDir, sessionPath)
	}
	keywords := getEnv("SEARCH_KEYWORDS", "golang engineer")
	excludeKeywords := getEnv("SEARCH_EXCLUDE_KEYWORDS", "")
	searchSeniority := getEnv("SEARCH_SENIORITY", "any")
	searchRemotePolicy := getEnv("SEARCH_REMOTE_POLICY", "strict-remote")
	searchSources := getEnv("SEARCH_SOURCES", "")
	maxCollect := getEnvInt("SCRAPER_MAX_COLLECT", 100)
	gupyBoards := getEnv("GUPY_COMPANY_URLS", "")
	greenhouseBoards := getEnv("GREENHOUSE_BOARDS", "")
	leverCompanies := getEnv("LEVER_COMPANIES", "")
	searchCountry := getEnv("SEARCH_COUNTRY", "BR")
	strategy := parseSearchStrategy(excludeKeywords, searchSeniority, searchRemotePolicy, searchSources)
	log.Printf("scraper: config keywords=%q country=%q", keywords, searchCountry)
	log.Printf("scraper: strategy seniority=%q remote=%q excludes=%d sources=%d", strategy.seniority, strategy.remotePolicy, len(strategy.excludeKeywords), len(strategy.allowedSources))
	log.Printf("scraper: max collect cap=%d", maxCollect)
	log.Printf("scraper: providers enabled -> linkedin=true gupy=%t greenhouse=%t lever=%t",
		strings.TrimSpace(gupyBoards) != "",
		strings.TrimSpace(greenhouseBoards) != "",
		strings.TrimSpace(leverCompanies) != "",
	)

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
		log.Println("scraper: continuing sem sessão; tentativa de coleta pública do LinkedIn")
	}

	page, err := ctx.NewPage()
	if err != nil {
		return fmt.Errorf("run: failed to open page: %w", err)
	}
	defer page.Close()

	vagas := make([]domain.Vaga, 0, 64)
	linkedinQueries := buildLinkedInSearchQueries(keywords)
	if len(linkedinQueries) == 0 {
		linkedinQueries = []string{keywords}
	}

	collectionGoal := maxCollect

	seenLinkedIn := map[string]struct{}{}

	linkedinTotal := 0
	// ── Etapas 5 e 6: navega e extrai vagas do LinkedIn por termo ───────────
	for _, query := range linkedinQueries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		for start := 0; start < collectionGoal; start += linkedInSearchPageSize {
			if len(vagas) >= collectionGoal {
				break
			}

			if err := NavigateToLinkedInSearchAtOffset(page, query, start); err != nil {
				log.Printf("scraper: aviso ao navegar LinkedIn com query %q start=%d: %v", query, start, err)
				break
			}

			vagasLinkedIn, extractErr := ExtractVagas(page)
			if extractErr != nil {
				log.Printf("scraper: aviso ao extrair LinkedIn com query %q start=%d: %v", query, start, extractErr)
				break
			}
			if len(vagasLinkedIn) == 0 {
				log.Printf("scraper: 0 vagas extraídas do LinkedIn para %q start=%d", query, start)
				break
			}

			pageAdded := 0
			for _, vaga := range vagasLinkedIn {
				key := dedupeKey(vaga)
				if key == "" {
					continue
				}
				if _, ok := seenLinkedIn[key]; ok {
					continue
				}
				seenLinkedIn[key] = struct{}{}
				vagas = append(vagas, vaga)
				pageAdded++
			}

			linkedinTotal += pageAdded
			log.Printf("scraper: %d vagas novas extraídas do LinkedIn para %q start=%d", pageAdded, query, start)
		}
	}
	log.Printf("scraper: total do LinkedIn antes da persistência: %d", linkedinTotal)

	if gupyBoards != "" {
		vagasGupy, gupyErr := ExtractGupyVagas(ctx, gupyBoards, keywords, searchCountry)
		if gupyErr != nil {
			log.Printf("scraper: aviso ao coletar Gupy: %v", gupyErr)
		} else {
			vagas = append(vagas, vagasGupy...)
			log.Printf("scraper: %d vagas extraídas da Gupy", len(vagasGupy))
		}
	} else {
		log.Printf("scraper: Gupy desativado (GUPY_COMPANY_URLS vazio)")
	}

	if greenhouseBoards != "" {
		vagasGreenhouse, ghErr := FetchGreenhouseVagas(greenhouseBoards, keywords, searchCountry)
		if ghErr != nil {
			log.Printf("scraper: aviso ao coletar Greenhouse: %v", ghErr)
		} else {
			vagas = append(vagas, vagasGreenhouse...)
			log.Printf("scraper: %d vagas extraídas da Greenhouse", len(vagasGreenhouse))
		}
	} else {
		log.Printf("scraper: Greenhouse desativado (GREENHOUSE_BOARDS vazio)")
	}

	if leverCompanies != "" {
		vagasLever, leverErr := FetchLeverVagas(leverCompanies, keywords, searchCountry)
		if leverErr != nil {
			log.Printf("scraper: aviso ao coletar Lever: %v", leverErr)
		} else {
			vagas = append(vagas, vagasLever...)
			log.Printf("scraper: %d vagas extraídas da Lever", len(vagasLever))
		}
	} else {
		log.Printf("scraper: Lever desativado (LEVER_COMPANIES vazio)")
	}

	log.Printf("scraper: total agregado de vagas antes da persistência: %d", len(vagas))
	vagasDedup := dedupeVagas(vagas)
	if len(vagasDedup) != len(vagas) {
		log.Printf("scraper: %d vagas duplicadas removidas antes de persistir", len(vagas)-len(vagasDedup))
	}
	filteredVagas, strategyStats := filterVagasByStrategyWithStats(vagasDedup, strategy)
	vagas = filteredVagas
	log.Printf(
		"scraper: strategy stats input=%d kept=%d dropped_source=%d dropped_exclude=%d dropped_seniority=%d dropped_remote=%d",
		strategyStats.InputCount,
		strategyStats.KeptCount,
		strategyStats.DroppedBySource,
		strategyStats.DroppedByExclude,
		strategyStats.DroppedBySeniority,
		strategyStats.DroppedByRemote,
	)
	log.Printf("scraper: total após estratégia: %d", len(vagas))

	vagas, droppedByRelevance := selectTopRelevantVagas(vagas, keywords, maxCollect)
	if droppedByRelevance > 0 {
		log.Printf("scraper: %d vagas removidas por baixa relevância keyword/role", droppedByRelevance)
	}
	if len(vagas) > maxCollect {
		log.Printf("scraper: aplicando corte final para cap=%d (antes=%d)", maxCollect, len(vagas))
		vagas = vagas[:maxCollect]
	}
	log.Printf("scraper: total final pronto para persistência: %d", len(vagas))

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

// getEnv retorna uma variável de ambiente ou um valor padrão.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 500 {
		return 500
	}
	return parsed
}

func keywordTerms(raw string) []string {
	queries := buildLinkedInSearchQueries(raw)
	out := make([]string, 0, len(queries)*3)
	seen := map[string]struct{}{}
	appendTerm := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" || len(term) < 3 {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}

	for _, query := range queries {
		appendTerm(query)
		for _, token := range strings.Fields(query) {
			appendTerm(token)
		}
	}

	if len(out) == 0 {
		for _, token := range strings.Fields(raw) {
			appendTerm(token)
		}
	}

	return out
}

func scoreVagaByKeywords(v domain.Vaga, terms []string) (titleScore int, keywordHits int, remoteBoost int) {
	if len(terms) == 0 {
		return 0, 0, 0
	}
	title := strings.ToLower(v.Titulo)
	body := strings.ToLower(v.Titulo + " " + v.Descricao + " " + v.Empresa)
	for _, term := range terms {
		if strings.Contains(title, term) {
			titleScore++
		}
		if strings.Contains(body, term) {
			keywordHits++
		}
	}
	if strings.Contains(body, "remote") || strings.Contains(body, "remoto") || strings.Contains(body, "home office") {
		remoteBoost = 1
	}
	return titleScore, keywordHits, remoteBoost
}

func selectTopRelevantVagas(vagas []domain.Vaga, keywords string, cap int) ([]domain.Vaga, int) {
	if len(vagas) == 0 {
		return vagas, 0
	}
	if cap <= 0 {
		cap = len(vagas)
	}

	terms := keywordTerms(keywords)
	ranked := make([]vagaRank, 0, len(vagas))
	dropped := 0
	for _, v := range vagas {
		titleScore, keywordHits, remoteBoost := scoreVagaByKeywords(v, terms)
		if len(terms) > 0 && titleScore == 0 && keywordHits < 2 {
			dropped++
			continue
		}
		ranked = append(ranked, vagaRank{vaga: v, titleScore: titleScore, keywordHits: keywordHits, remoteBoost: remoteBoost})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].titleScore != ranked[j].titleScore {
			return ranked[i].titleScore > ranked[j].titleScore
		}
		if ranked[i].keywordHits != ranked[j].keywordHits {
			return ranked[i].keywordHits > ranked[j].keywordHits
		}
		if ranked[i].remoteBoost != ranked[j].remoteBoost {
			return ranked[i].remoteBoost > ranked[j].remoteBoost
		}
		return strings.ToLower(ranked[i].vaga.Titulo) < strings.ToLower(ranked[j].vaga.Titulo)
	})

	if len(ranked) > cap {
		dropped += len(ranked) - cap
		ranked = ranked[:cap]
	}

	out := make([]domain.Vaga, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.vaga)
	}

	return out, dropped
}

func buildLinkedInSearchQueries(raw string) []string {
	parts := parseCSV(raw)
	if len(parts) == 0 {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	queries := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		query := strings.TrimSpace(part)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		queries = append(queries, query)
	}

	return queries
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

func parseSearchStrategy(excludeCSV, seniorityRaw, remotePolicyRaw, sourcesCSV string) searchStrategy {
	strategy := searchStrategy{
		excludeKeywords: map[string]struct{}{},
		seniority:       normalizeSeniority(seniorityRaw),
		remotePolicy:    normalizeRemotePolicy(remotePolicyRaw),
		allowedSources:  map[string]struct{}{},
	}

	for _, term := range parseCSV(excludeCSV) {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		strategy.excludeKeywords[normalized] = struct{}{}
	}

	for _, source := range parseCSV(sourcesCSV) {
		normalized := strings.ToLower(strings.TrimSpace(source))
		switch normalized {
		case "linkedin", "gupy", "greenhouse", "lever":
			strategy.allowedSources[normalized] = struct{}{}
		}
	}

	return strategy
}

func normalizeSeniority(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "junior", "mid", "senior", "staff", "lead":
		return v
	default:
		return "any"
	}
}

func normalizeRemotePolicy(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "any", "remote-first", "strict-remote":
		return v
	default:
		return "strict-remote"
	}
}

func filterVagasByStrategy(vagas []domain.Vaga, strategy searchStrategy) []domain.Vaga {
	filtered, _ := filterVagasByStrategyWithStats(vagas, strategy)
	return filtered
}

func filterVagasByStrategyWithStats(vagas []domain.Vaga, strategy searchStrategy) ([]domain.Vaga, strategyFilterStats) {
	stats := strategyFilterStats{InputCount: len(vagas)}
	if len(vagas) == 0 {
		return vagas, stats
	}

	filtered := make([]domain.Vaga, 0, len(vagas))
	for _, v := range vagas {
		if !sourceAllowed(v, strategy.allowedSources) {
			stats.DroppedBySource++
			continue
		}
		if containsExcludedKeyword(v, strategy.excludeKeywords) {
			stats.DroppedByExclude++
			continue
		}
		if !matchesSeniorityStrategy(v, strategy.seniority) {
			stats.DroppedBySeniority++
			continue
		}
		if !matchesRemotePolicy(v, strategy.remotePolicy) {
			stats.DroppedByRemote++
			continue
		}
		filtered = append(filtered, v)
	}

	stats.KeptCount = len(filtered)
	return filtered, stats
}

func inferVagaSource(v domain.Vaga) string {
	blob := strings.ToLower(strings.TrimSpace(v.URL))
	switch {
	case strings.Contains(blob, "linkedin.com"):
		return "linkedin"
	case strings.Contains(blob, "gupy.io"):
		return "gupy"
	case strings.Contains(blob, "greenhouse.io") || strings.Contains(blob, "boards.greenhouse"):
		return "greenhouse"
	case strings.Contains(blob, "lever.co") || strings.Contains(blob, "jobs.lever"):
		return "lever"
	default:
		return "other"
	}
}

func sourceAllowed(v domain.Vaga, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[inferVagaSource(v)]
	return ok
}

func containsExcludedKeyword(v domain.Vaga, excludes map[string]struct{}) bool {
	if len(excludes) == 0 {
		return false
	}
	blob := strings.ToLower(v.Titulo + " " + v.Descricao + " " + v.Empresa)
	for term := range excludes {
		if term != "" && strings.Contains(blob, term) {
			return true
		}
	}
	return false
}

func matchesSeniorityStrategy(v domain.Vaga, seniority string) bool {
	if seniority == "any" {
		return true
	}
	blob := strings.ToLower(v.Titulo + " " + v.Descricao)
	has := func(terms ...string) bool {
		for _, term := range terms {
			if strings.Contains(blob, term) {
				return true
			}
		}
		return false
	}

	switch seniority {
	case "junior":
		return has("junior", "jr", "entry")
	case "mid":
		return has("mid", "middle", "pleno", "intermediate")
	case "senior":
		return has("senior", "sr", "sênior")
	case "staff":
		return has("staff", "principal")
	case "lead":
		return has("lead", "tech lead", "líder", "lider")
	default:
		return true
	}
}

func matchesRemotePolicy(v domain.Vaga, remotePolicy string) bool {
	if remotePolicy == "any" {
		return true
	}
	blob := strings.ToLower(v.Titulo + " " + v.Descricao)
	contains := func(terms ...string) bool {
		for _, term := range terms {
			if strings.Contains(blob, term) {
				return true
			}
		}
		return false
	}

	remoteHint := contains("remote", "remoto", "home office", "work from home", "wfh")
	onsiteHint := contains("onsite", "on-site", "presencial")
	hybridHint := contains("hybrid", "híbrido", "hibrido")

	switch remotePolicy {
	case "strict-remote":
		return remoteHint && !onsiteHint && !hybridHint
	case "remote-first":
		if onsiteHint && !remoteHint {
			return false
		}
		return true
	default:
		return true
	}
}
