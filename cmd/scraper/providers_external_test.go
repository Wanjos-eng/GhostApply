package main

import (
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
