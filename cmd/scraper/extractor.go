// Package main — lógica de extração de cards de vagas do scraper GhostApply.
//
// # Intenção
// Navega até uma busca do LinkedIn filtrada para remoto e extrai os dados
// estruturados de cada card usando seletores de acessibilidade.
package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/playwright-community/playwright-go"
)

const linkedInJobsBaseURL = "https://www.linkedin.com/jobs/search/"
const linkedInSearchPageSize = 25

func normalizeLinkedInURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed
	}

	base, _ := url.Parse("https://www.linkedin.com")
	rel, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if rel.IsAbs() {
		return trimmed
	}

	return base.ResolveReference(rel).String()
}

// NavigateToLinkedInSearch abre uma busca do LinkedIn filtrada para 100% remoto.
//
// # Intenção
// f_WT=2 é o parâmetro do LinkedIn para vagas remotas.
// Fixar esse filtro evita coletar vagas presenciais por acidente.
func NavigateToLinkedInSearch(page playwright.Page, keywords string) error {
	return NavigateToLinkedInSearchAtOffset(page, keywords, 0)
}

// NavigateToLinkedInSearchAtOffset abre uma busca do LinkedIn na página indicada.
func NavigateToLinkedInSearchAtOffset(page playwright.Page, keywords string, start int) error {
	params := url.Values{}
	params.Set("keywords", keywords)
	params.Set("f_WT", "2")       // Filtro obrigatório de vaga remota.
	params.Set("f_TPR", "r86400") // Filtro de recorte nas últimas 24h.
	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}

	target := linkedInJobsBaseURL + "?" + params.Encode()

	if _, err := page.Goto(target, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return fmt.Errorf("NavigateToLinkedInSearch: navigation failed: %w", err)
	}

	HumanSleep()
	return nil
}

func linkedInSearchURL(keywords string, start int) string {
	params := url.Values{}
	params.Set("keywords", keywords)
	params.Set("f_WT", "2")
	params.Set("f_TPR", "r86400")
	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}
	return linkedInJobsBaseURL + "?" + params.Encode()
}

// ExtractVagas extrai todos os cards visíveis da página atual de resultados.
//
// # Concorrência
// Cada card é processado de forma sequencial porque a página do Playwright não
// é thread-safe. Se uma vaga falhar, o restante continua normalmente.
func ExtractVagas(page playwright.Page) ([]domain.Vaga, error) {
	selectors := []string{
		"ul.jobs-search__results-list li",
		"li:has(a[href*='/jobs/view/'])",
		"li.jobs-search-results__list-item",
		"div.jobs-search-results-list li",
	}

	var cards []playwright.Locator
	for _, selector := range selectors {
		locators, err := page.Locator(selector).All()
		if err != nil || len(locators) == 0 {
			continue
		}
		cards = locators
		break
	}

	if len(cards) == 0 {
		return nil, nil
	}

	// Processamento sequencial: a página do Playwright não é thread-safe.
	// Chamadas concorrentes de Click()/InnerText() na mesma página causam corrida.
	var results []domain.Vaga
	for _, card := range cards {
		vaga, err := extractSingleCard(page, card)
		if err != nil {
			continue
		}
		results = append(results, vaga)
	}

	return results, nil
}

// extractSingleCard extrai uma Vaga a partir de um locator de card.
//
// Prioriza seletores de acessibilidade quando possível para resistir a
// mudanças frequentes de classe no LinkedIn.
func extractSingleCard(page playwright.Page, card playwright.Locator) (domain.Vaga, error) {
	titulo := ""
	titleSelectors := []string{
		"h3",
		"[role='heading']",
		".base-search-card__title",
		"span[aria-hidden='true']",
	}
	for _, selector := range titleSelectors {
		loc := card.Locator(selector).First()
		if count, _ := loc.Count(); count == 0 {
			continue
		}
		if text, err := loc.InnerText(); err == nil {
			if normalized := strings.TrimSpace(text); normalized != "" {
				titulo = normalized
				break
			}
		}
	}
	if titulo == "" {
		if text, err := card.InnerText(); err == nil {
			lines := strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
			for _, line := range lines {
				normalized := strings.TrimSpace(line)
				if normalized != "" {
					titulo = normalized
					break
				}
			}
		}
	}
	if titulo == "" {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get titulo")
	}

	empresa := ""
	companySelectors := []string{
		".base-search-card__subtitle",
		".job-card-container__primary-description",
		"[data-test-id='job-card-company']",
	}
	for _, selector := range companySelectors {
		loc := card.Locator(selector).First()
		if count, _ := loc.Count(); count == 0 {
			continue
		}
		if text, err := loc.InnerText(); err == nil {
			if normalized := strings.TrimSpace(text); normalized != "" {
				empresa = normalized
				break
			}
		}
	}
	if empresa == "" {
		if text, err := card.InnerText(); err == nil {
			lines := strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
			for i, line := range lines {
				normalized := strings.TrimSpace(line)
				if normalized == "" || strings.EqualFold(normalized, titulo) {
					continue
				}
				if i == 0 {
					continue
				}
				empresa = normalized
				break
			}
		}
	}
	if empresa == "" {
		empresa = "LinkedIn"
	}

	jobURL := ""
	for _, selector := range []string{"a.base-card__full-link", "a[href*='/jobs/view/']"} {
		linkEl := card.Locator(selector).First()
		if count, _ := linkEl.Count(); count == 0 {
			continue
		}
		if href, err := linkEl.GetAttribute("href"); err == nil && strings.TrimSpace(href) != "" {
			jobURL = normalizeLinkedInURL(href)
			break
		}
	}
	if jobURL == "" {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get job URL")
	}

	// Clica no card para carregar o painel de detalhes e extrair a descrição.
	linkEl := card.Locator("a.base-card__full-link").First()
	if count, _ := linkEl.Count(); count == 0 {
		linkEl = card.Locator("a[href*='/jobs/view/']").First()
	}
	if err := linkEl.Click(); err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: failed to click card: %w", err)
	}
	HumanSleep()

	descricao := ""
	for _, selector := range []string{
		".show-more-less-html__markup",
		"section[class*='show-more-less']",
		"[data-test-id='job-description']",
		"main",
	} {
		descEl := page.Locator(selector).First()
		if count, _ := descEl.Count(); count == 0 {
			continue
		}
		if text, err := descEl.InnerText(); err == nil {
			if normalized := strings.TrimSpace(text); normalized != "" {
				descricao = normalized
				break
			}
		}
	}

	// Extrai dados do recrutador se estiver disponível no painel de detalhes.
	var recrutadorNome, recrutadorPerfil *string
	hiringTeamEl := page.Locator(".hirer-card__hirer-information").First()
	if count, _ := hiringTeamEl.Count(); count > 0 {
		nameEl := hiringTeamEl.Locator("strong, .text-heading-small").First()
		if text, err := nameEl.InnerText(); err == nil && text != "" {
			recrutadorNome = &text
		}

		linkEl := hiringTeamEl.Locator("a").First()
		if href, err := linkEl.GetAttribute("href"); err == nil && href != "" {
			normalized := normalizeLinkedInURL(href)
			recrutadorPerfil = &normalized
		}
	}

	return domain.Vaga{
		Titulo:           titulo,
		Empresa:          empresa,
		URL:              jobURL,
		Descricao:        descricao,
		Status:           domain.StatusPendente,
		RecrutadorNome:   recrutadorNome,
		RecrutadorPerfil: recrutadorPerfil,
	}, nil
}
