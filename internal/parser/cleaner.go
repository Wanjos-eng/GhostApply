// Package parser fornece utilitários de sanitização de texto para HTML bruto coletado de portais.
//
// # Intenção
// Descrições de vaga vêm com tags HTML, rastreadores, scripts embutidos e
// às vezes conteúdo adversarial para manipular LLMs (prompt injection).
// Este pacote remove agressivamente tudo que não seja texto legível.
//
// # Restrição (SecOps)
// Tudo isso precisa sair antes do texto entrar no pipeline de IA:
//   - blocos <script> e <style> (vetores de execução de código)
//   - endereços de email (vazamento de PII e isca de prompt injection)
//   - links e URLs soltas (redirecionamento e phishing)
//   - tags HTML residuais
package parser

import (
	"errors"
	"regexp"
	"strings"
)

// ── regex compiladas (nível de pacote = compiladas uma vez) ─────────────────

var (
	// Remove blocos <script>...</script>, inclusive conteúdo multilinha.
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

	// Remove blocos <style>...</style>.
	reStyle = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)

	// Remove todas as tags HTML restantes.
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)

	// Remove emails, que são PII e ponto comum de prompt injection.
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// Remove URLs (http/https/ftp e links com www.).
	reURL = regexp.MustCompile(`(?i)(https?://|ftp://|www\.)\S+`)

	// Colapsa múltiplos espaços em um único espaço.
	reWhitespace = regexp.MustCompile(`\s+`)
)

// ErrEmptyResult é retornado quando a sanitização não deixa texto legível.
// O chamador deve tratar isso como sinal para descartar a descrição.
var ErrEmptyResult = errors.New("parser: sanitised text is empty — raw input had no usable content")

// Clean sanitiza HTML bruto de um portal de vagas e devolve texto puro seguro para a IA.
//
// A ordem de remoção importa:
//  1. blocos de script/style primeiro, porque podem conter URLs/emails
//  2. todas as tags HTML
//  3. emails e URLs
//  4. normalização de espaços em branco
func Clean(raw string) (string, error) {
	s := raw
	s = reScript.ReplaceAllString(s, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reHTMLTag.ReplaceAllString(s, " ")
	s = reEmail.ReplaceAllString(s, " ")
	s = reURL.ReplaceAllString(s, " ")
	s = reWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	if s == "" {
		return "", ErrEmptyResult
	}

	return s, nil
}
