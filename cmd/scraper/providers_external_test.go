package main

import (
	"os"
	"strings"
	"testing"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
)

func TestParseCSV(t *testing.T) {
	in := " a ,b, , c ,,"
	got := parseCSV(in)
	if len(got) != 3 {
		t.Fatalf("esperava 3 itens, veio %d", len(got))
	}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("parse inesperado: %#v", got)
	}
}

func TestBuildLinkedInSearchQueries(t *testing.T) {
	got := buildLinkedInSearchQueries(" software engineer , backend,backend, go ")
	want := []string{"software engineer", "backend", "go"}
	if len(got) != len(want) {
		t.Fatalf("esperava %d queries, veio %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("query[%d] = %q, want %q (all=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestLinkedInSearchURL(t *testing.T) {
	got := linkedInSearchURL("Go Engineer", 25)
	if got == "" {
		t.Fatal("linkedInSearchURL não pode ser vazio")
	}
	if !strings.Contains(got, "keywords=Go+Engineer") || !strings.Contains(got, "f_WT=2") || !strings.Contains(got, "f_TPR=r86400") || !strings.Contains(got, "start=25") {
		t.Fatalf("linkedInSearchURL gerou URL inesperada: %s", got)
	}
}

func TestNormalizeGupyBoardURL(t *testing.T) {
	cases := map[string]string{
		"empresa.gupy.io":              "https://empresa.gupy.io/jobs",
		"https://empresa.gupy.io":      "https://empresa.gupy.io/jobs",
		"https://empresa.gupy.io/jobs": "https://empresa.gupy.io/jobs",
	}

	for in, expected := range cases {
		if got := normalizeGupyBoardURL(in); got != expected {
			t.Fatalf("normalizeGupyBoardURL(%q) = %q, esperado %q", in, got, expected)
		}
	}
}

func TestMatchesKeywords(t *testing.T) {
	if !matchesKeywords("Senior Go Backend Engineer", "", "go,rust") {
		t.Fatalf("deveria casar por palavra-chave")
	}
	if matchesKeywords("Marketing Analyst", "", "go,rust") {
		t.Fatalf("não deveria casar para palavra-chave fora da área")
	}
	if !matchesKeywords("Qualquer título", "qualquer descrição", "") {
		t.Fatalf("com keyword vazia deve aceitar")
	}
}

func TestMatchesCountryBR(t *testing.T) {
	if !matchesCountry("vaga remota para brasil", "BR") {
		t.Fatalf("deveria aceitar vaga BR")
	}
	if matchesCountry("fully remote role for global candidates", "BR") {
		t.Fatalf("não deveria aceitar remoto genérico sem contexto BR")
	}
	if matchesCountry("onsite madrid spain", "BR") {
		t.Fatalf("não deveria aceitar vaga claramente fora de BR")
	}
}

func TestDedupeVagas(t *testing.T) {
	in := []domain.Vaga{
		{URL: "https://exemplo.com/jobs/1", Titulo: "Backend", Empresa: "Acme"},
		{URL: "https://exemplo.com/jobs/1", Titulo: "Backend duplicada", Empresa: "Acme"},
		{URL: "", Titulo: "Dev Go", Empresa: "Foo"},
		{URL: "", Titulo: "Dev Go", Empresa: "Foo"},
		{URL: "https://exemplo.com/jobs/2", Titulo: "Platform", Empresa: "Bar"},
	}

	out := dedupeVagas(in)
	if len(out) != 3 {
		t.Fatalf("esperava 3 vagas após dedupe, veio %d", len(out))
	}
}

func TestDedupeKeyFallback(t *testing.T) {
	v := domain.Vaga{Titulo: " Senior Go Engineer ", Empresa: " Acme "}
	key := dedupeKey(v)
	if key == "" {
		t.Fatalf("dedupeKey não deveria ser vazio para título+empresa")
	}

	v2 := domain.Vaga{}
	if dedupeKey(v2) != "" {
		t.Fatalf("dedupeKey deveria ser vazio quando não há URL nem metadados")
	}
}

func TestParseSearchStrategy(t *testing.T) {
	strategy := parseSearchStrategy("estagio,estágio", "Senior", "strict-remote", "linkedin,gupy,foo")
	if strategy.seniority != "senior" {
		t.Fatalf("seniority inesperado: %q", strategy.seniority)
	}
	if strategy.remotePolicy != "strict-remote" {
		t.Fatalf("remote policy inesperada: %q", strategy.remotePolicy)
	}
	if len(strategy.excludeKeywords) != 2 {
		t.Fatalf("esperava 2 exclusões, veio %d", len(strategy.excludeKeywords))
	}
	if len(strategy.allowedSources) != 2 {
		t.Fatalf("esperava 2 fontes permitidas, veio %d", len(strategy.allowedSources))
	}
}

func TestFilterVagasByStrategy(t *testing.T) {
	vagas := []domain.Vaga{
		{Titulo: "Senior Go Engineer", Descricao: "remote role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/1"},
		{Titulo: "Junior Go Engineer", Descricao: "remote role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/2"},
		{Titulo: "Senior Java Engineer", Descricao: "hybrid role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/3"},
		{Titulo: "Senior Go Engineer", Descricao: "remote role", Empresa: "InternCo", URL: "https://www.linkedin.com/jobs/view/4"},
	}

	strategy := parseSearchStrategy("intern", "senior", "strict-remote", "linkedin")
	filtered := filterVagasByStrategy(vagas, strategy)
	if len(filtered) != 1 {
		t.Fatalf("esperava 1 vaga após filtros, veio %d (%#v)", len(filtered), filtered)
	}
	if !strings.Contains(strings.ToLower(filtered[0].Titulo), "senior") {
		t.Fatalf("vaga restante deveria manter senioridade alvo")
	}
}

func TestFilterVagasByStrategyWithStats(t *testing.T) {
	vagas := []domain.Vaga{
		{Titulo: "Senior Go Engineer", Descricao: "remote role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/1"},
		{Titulo: "Senior Go Engineer", Descricao: "remote role", Empresa: "InternCo", URL: "https://www.linkedin.com/jobs/view/2"},
		{Titulo: "Junior Go Engineer", Descricao: "remote role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/3"},
		{Titulo: "Senior Go Engineer", Descricao: "hybrid role", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/4"},
		{Titulo: "Senior Go Engineer", Descricao: "remote role", Empresa: "Acme", URL: "https://jobs.lever.co/acme/1"},
	}

	strategy := parseSearchStrategy("intern", "senior", "strict-remote", "linkedin")
	filtered, stats := filterVagasByStrategyWithStats(vagas, strategy)

	if len(filtered) != 1 {
		t.Fatalf("esperava 1 vaga após filtros, veio %d", len(filtered))
	}
	if stats.InputCount != 5 {
		t.Fatalf("input count inesperado: %d", stats.InputCount)
	}
	if stats.KeptCount != 1 {
		t.Fatalf("kept count inesperado: %d", stats.KeptCount)
	}
	if stats.DroppedByExclude != 1 {
		t.Fatalf("dropped exclude inesperado: %d", stats.DroppedByExclude)
	}
	if stats.DroppedBySeniority != 1 {
		t.Fatalf("dropped seniority inesperado: %d", stats.DroppedBySeniority)
	}
	if stats.DroppedByRemote != 1 {
		t.Fatalf("dropped remote inesperado: %d", stats.DroppedByRemote)
	}
	if stats.DroppedBySource != 1 {
		t.Fatalf("dropped source inesperado: %d", stats.DroppedBySource)
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("SCRAPER_MAX_COLLECT", "120")
	if got := getEnvInt("SCRAPER_MAX_COLLECT", 100); got != 120 {
		t.Fatalf("esperava 120, veio %d", got)
	}

	t.Setenv("SCRAPER_MAX_COLLECT", "0")
	if got := getEnvInt("SCRAPER_MAX_COLLECT", 100); got != 100 {
		t.Fatalf("valor inválido deveria cair para fallback, veio %d", got)
	}

	t.Setenv("SCRAPER_MAX_COLLECT", "9999")
	if got := getEnvInt("SCRAPER_MAX_COLLECT", 100); got != 500 {
		t.Fatalf("valor deveria respeitar teto de segurança 500, veio %d", got)
	}

	if err := os.Unsetenv("SCRAPER_MAX_COLLECT"); err != nil {
		t.Fatalf("falha ao limpar env: %v", err)
	}
	if got := getEnvInt("SCRAPER_MAX_COLLECT", 90); got != 90 {
		t.Fatalf("env ausente deveria usar fallback, veio %d", got)
	}
}

func TestSelectTopRelevantVagas(t *testing.T) {
	vagas := []domain.Vaga{
		{Titulo: "Senior Go Backend Engineer", Descricao: "Remote role with Go and distributed systems", Empresa: "Acme", URL: "https://www.linkedin.com/jobs/view/1"},
		{Titulo: "Backend Platform Engineer", Descricao: "Golang backend, remote-first", Empresa: "Beta", URL: "https://www.linkedin.com/jobs/view/2"},
		{Titulo: "Marketing Analyst", Descricao: "Growth and performance marketing", Empresa: "Gamma", URL: "https://www.linkedin.com/jobs/view/3"},
		{Titulo: "Java Developer", Descricao: "Hybrid onsite role", Empresa: "Delta", URL: "https://www.linkedin.com/jobs/view/4"},
	}

	selected, dropped := selectTopRelevantVagas(vagas, "go backend", 2)
	if len(selected) != 2 {
		t.Fatalf("esperava 2 vagas após seleção, veio %d", len(selected))
	}
	if dropped <= 0 {
		t.Fatalf("esperava ao menos 1 vaga descartada por baixa relevância")
	}

	for _, v := range selected {
		blob := strings.ToLower(v.Titulo + " " + v.Descricao)
		if !strings.Contains(blob, "go") && !strings.Contains(blob, "backend") {
			t.Fatalf("vaga irrelevante não deveria permanecer: %+v", v)
		}
	}
}
