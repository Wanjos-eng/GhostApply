// Package main — job card extraction logic for the GhostApply scraper.
//
// # Intent
// Navigate to a LinkedIn job search page filtered for 100% remote roles,
// then extract structured Vaga data from each card using accessibility selectors.
// Cards are processed concurrently via goroutines to reduce wall-clock time.
package main

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/playwright-community/playwright-go"
)

const linkedInJobsBaseURL = "https://www.linkedin.com/jobs/search/"

// NavigateToLinkedInSearch navigates to a LinkedIn job search filtered to 100% remote.
//
// # Intent (Task 18)
// f_WT=2 is LinkedIn's query param for "Remote" work type.
// Hardcoding it prevents accidental scraping of on-site roles.
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

// ExtractVagas extracts all visible job cards from the current search results page.
//
// # Concurrency (Task 20)
// Each card is processed in its own goroutine. A Mutex protects the shared results
// slice. Errors per-card are soft-logged (not fatal) — partial results are better
// than aborting the entire run due to one malformed card.
func ExtractVagas(page playwright.Page) ([]domain.Vaga, error) {
	// Task 20: locate all job cards on the page
	cards, err := page.Locator("ul.jobs-search__results-list li").All()
	if err != nil {
		return nil, fmt.Errorf("ExtractVagas: failed to locate job cards: %w", err)
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []domain.Vaga
	)

	for _, card := range cards {
		wg.Add(1)
		go func(card playwright.Locator) {
			defer wg.Done()

			vaga, err := extractSingleCard(page, card)
			if err != nil {
				// Soft failure: skip malformed card, do not abort pipeline
				return
			}

			mu.Lock()
			results = append(results, vaga)
			mu.Unlock()
		}(card)
	}

	wg.Wait()
	return results, nil
}

// extractSingleCard extracts a single Vaga from a job card locator.
//
// Uses accessibility selectors (GetByRole) where possible — more resilient
// to LinkedIn's frequent class-name obfuscation.
func extractSingleCard(page playwright.Page, card playwright.Locator) (domain.Vaga, error) {
	// Task 21: title via accessibility role — resilient to class churn
	tituloEl := card.GetByRole("heading", playwright.LocatorGetByRoleOptions{
		Level: playwright.Int(3),
	})
	titulo, err := tituloEl.InnerText()
	if err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get titulo: %w", err)
	}

	// Task 22: company name
	empresaEl := card.Locator(".base-search-card__subtitle")
	empresa, err := empresaEl.InnerText()
	if err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get empresa: %w", err)
	}

	// Task 22: direct job URL from the card anchor
	linkEl := card.Locator("a.base-card__full-link")
	jobURL, err := linkEl.GetAttribute("href")
	if err != nil || jobURL == "" {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: cannot get job URL")
	}

	// Task 23: click card to load the detail panel, then extract description
	if err := linkEl.Click(); err != nil {
		return domain.Vaga{}, fmt.Errorf("extractSingleCard: failed to click card: %w", err)
	}
	HumanSleep()

	descEl := page.Locator(".show-more-less-html__markup")
	descricao, err := descEl.InnerText()
	if err != nil {
		descricao = "" // description is optional; continue without it
	}

	return domain.Vaga{
		Titulo:    titulo,
		Empresa:   empresa,
		URL:       jobURL,
		Descricao: descricao,
		Status:    domain.StatusPendente,
	}, nil
}
