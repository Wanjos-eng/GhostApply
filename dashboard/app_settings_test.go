package main

import (
	"bytes"
	"testing"
)

func TestMergeSettingsEnvPreservesExistingSecrets(t *testing.T) {
	existing := map[string]string{
		"COHERE_API_KEY": "cohere-old",
		"GROQ_API_KEY":   "groq-old",
		"GEMINI_API_KEY": "gemini-old",
		"IMAP_PASS":      "imap-old",
		"SOME_OTHER_KEY": "keep-me",
	}

	cfg := SettingsDTO{
		ImapServer: " imap.gmail.com:993 ",
		ImapUser:   "  user@example.com ",
		// Secrets vazios não devem sobrescrever valores existentes.
		CohereAPIKey: "",
		GroqAPIKey:   "",
		GeminiAPIKey: "",
		ImapPass:     "",
	}

	got := mergeSettingsEnv(existing, cfg)

	if got["COHERE_API_KEY"] != "cohere-old" {
		t.Fatalf("COHERE_API_KEY deveria ser preservada")
	}
	if got["GROQ_API_KEY"] != "groq-old" {
		t.Fatalf("GROQ_API_KEY deveria ser preservada")
	}
	if got["GEMINI_API_KEY"] != "gemini-old" {
		t.Fatalf("GEMINI_API_KEY deveria ser preservada")
	}
	if got["IMAP_PASS"] != "imap-old" {
		t.Fatalf("IMAP_PASS deveria ser preservada")
	}
	if got["IMAP_SERVER"] != "imap.gmail.com:993" {
		t.Fatalf("IMAP_SERVER deveria ser trimado")
	}
	if got["IMAP_USER"] != "user@example.com" {
		t.Fatalf("IMAP_USER deveria ser trimado")
	}
	if got["SOME_OTHER_KEY"] != "keep-me" {
		t.Fatalf("chave não relacionada não deveria ser perdida")
	}
}

func TestMergeSettingsEnvUpdatesProvidedSecrets(t *testing.T) {
	existing := map[string]string{}
	cfg := SettingsDTO{
		CohereAPIKey: "cohere-new",
		GroqAPIKey:   "groq-new",
		GeminiAPIKey: "gemini-new",
		ImapPass:     "imap-new",
	}

	got := mergeSettingsEnv(existing, cfg)

	if got["COHERE_API_KEY"] != "cohere-new" {
		t.Fatalf("COHERE_API_KEY deveria ser atualizada")
	}
	if got["GROQ_API_KEY"] != "groq-new" {
		t.Fatalf("GROQ_API_KEY deveria ser atualizada")
	}
	if got["GEMINI_API_KEY"] != "gemini-new" {
		t.Fatalf("GEMINI_API_KEY deveria ser atualizada")
	}
	if got["IMAP_PASS"] != "imap-new" {
		t.Fatalf("IMAP_PASS deveria ser atualizada")
	}
}

func TestBuildGeminiRequestUsesAPIKeyHeader(t *testing.T) {
	payload := []byte(`{"contents":[{"parts":[{"text":"ping"}]}]}`)
	req, err := buildGeminiRequest("gemini-2.0-flash", "test-key", payload)
	if err != nil {
		t.Fatalf("buildGeminiRequest retornou erro: %v", err)
	}

	if req.URL.RawQuery != "" {
		t.Fatalf("requisição não deve usar query string para chave de API")
	}
	if got := req.Header.Get("x-goog-api-key"); got != "test-key" {
		t.Fatalf("header x-goog-api-key inválido: %q", got)
	}
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(req.Body); err != nil {
		t.Fatalf("falha ao ler corpo da requisição: %v", err)
	}
	if body.Len() == 0 {
		t.Fatalf("corpo da requisição não deveria estar vazio")
	}
}
