// Package main — lógica de extração de cards de vagas do scraper GhostApply.
//
// # Intenção
// Navega até uma busca do LinkedIn filtrada para remoto e extrai os dados
// estruturados de cada card usando seletores de acessibilidade.
package main

import (
	"fmt"
	"net/url"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/playwright-community/playwright-go"
)

const linkedInJobsBaseURL = "https://www.linkedin.com/jobs/search/"

// NavigateToLinkedInSearch abre uma busca do LinkedIn filtrada para 100% remoto.
//
// # Intenção (Tarefa 18)
// f_WT=2 é o parâmetro do LinkedIn para vagas remotas.
// Fixar esse filtro evita coletar vagas presenciais por acidente.
func NavigateToLinkedInSearch(page playwright.Page, keywords string) error {
	params := url.Values{}
	params.Set("keywords", keywords)
	params.Set("f_WT", "2")   // 100% Remote — mandatory filter
	params.Set("f_TPR", "r86400") // Last 24 hours — freshness filter

	target := linkedInJobsBaseURL + "?" + params.Encode()

	if _, err := page.Goto(target, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("NavigateToLinkedInSearch: navigation failed: %w", err)
	}

	HumanSleep()
	return nil
}

// ExtractVagas extrai todos os cards visíveis da página atual de resultados.
//
// # Concorrência (Tarefa 20)
// Cada card é processado de forma sequencial porque a página do Playwright não
// é thread-safe. Se uma vaga falhar, o restante continua normalmente.
func ExtractVagas(page playwright.Page) ([]domain.Vaga, error) {
	// Localiza todos os cards de vaga na página.
	cards, err := page.Locator("ul.jobs-search__results-list li").All()
	if err != nil {
		return nil, fmt.Errorf("ExtractVagas: failed to locate job cards: %w", err)
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
	// Título via role de acessibilidade, mais resiliente a mudanças de classe.
	tituloEl := card.GetByRole("heading", playwright.LocatorGetByRoleOptions{
		Level: playwright.Int(3),
	})
	titulo, err := tituloEl.InnerText()
	if err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get titulo: %w", err)
	}

	// Nome da empresa.
	empresaEl := card.Locator(".base-search-card__subtitle")
	empresa, err := empresaEl.InnerText()
	if err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get empresa: %w", err)
	}

	// URL direta da vaga a partir do link do card.
	linkEl := card.Locator("a.base-card__full-link")
	jobURL, err := linkEl.GetAttribute("href")
	if err != nil || jobURL == "" {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get job URL")
	}

	// Clica no card para carregar o painel de detalhes e extrair a descrição.
	if err := linkEl.Click(); err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: failed to click card: %w", err)
	}
	HumanSleep()

	descEl := page.Locator(".show-more-less-html__markup")
	descricao, err := descEl.InnerText()
	if err != nil {
			descricao = "" // a descrição é opcional; seguimos sem ela
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
			recrutadorPerfil = &href
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
