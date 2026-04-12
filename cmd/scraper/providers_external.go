package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/playwright-community/playwright-go"
)

// ExtractGupyVagas coleta vagas públicas em páginas de carreira da Gupy.
// Espera uma lista CSV de URLs de empresas em GUPY_COMPANY_URLS.
func ExtractGupyVagas(ctx playwright.BrowserContext, boardsCSV, keywords, country string) ([]domain.Vaga, error) {
	boards := parseCSV(boardsCSV)
	if len(boards) == 0 {
		return nil, nil
	}

	var vagas []domain.Vaga
	seen := map[string]struct{}{}

	for _, board := range boards {
		page, err := ctx.NewPage()
		if err != nil {
			return vagas, fmt.Errorf("ExtractGupyVagas: falha ao abrir página: %w", err)
		}

		listURL := normalizeGupyBoardURL(board)
		if _, err := page.Goto(listURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateNetworkidle}); err != nil {
			page.Close()
			continue
		}
		HumanSleep()

		anchors, err := page.Locator("a[href*='/jobs/']").All()
		if err != nil {
			page.Close()
			continue
		}

		for _, a := range anchors {
			href, _ := a.GetAttribute("href")
			if href == "" {
				continue
			}

			jobURL := absolutizeURL(listURL, href)
			if _, ok := seen[jobURL]; ok {
				continue
			}

			titleRaw, _ := a.InnerText()
			title := normalizeSpace(titleRaw)
			if title == "" {
				continue
			}

			if !matchesKeywords(title, "", keywords) {
				continue
			}

			desc := fetchGupyDescription(ctx, jobURL)
			_ = country // Para Gupy, a cobertura geográfica é controlada pelas boards informadas.

			seen[jobURL] = struct{}{}
			vagas = append(vagas, domain.Vaga{
				Titulo:    title,
				Empresa:   companyFromURL(jobURL, "gupy"),
				URL:       jobURL,
				Descricao: desc,
				Status:    domain.StatusPendente,
			})
		}

		page.Close()
	}

	return vagas, nil
}

func fetchGupyDescription(ctx playwright.BrowserContext, jobURL string) string {
	page, err := ctx.NewPage()
	if err != nil {
		return ""
	}
	defer page.Close()

	if _, err := page.Goto(jobURL, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return ""
	}

	selectors := []string{
		"[data-testid='job-description']",
		".job-description",
		"section[class*='description']",
		"main",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel).First()
		if count, _ := loc.Count(); count == 0 {
			continue
		}
		text, _ := loc.InnerText()
		text = normalizeSpace(text)
		if text != "" {
			return text
		}
	}

	return ""
}

// FetchGreenhouseVagas consulta a API pública do Greenhouse por board.
// Usa GREENHOUSE_BOARDS em formato CSV com os slugs das empresas.
func FetchGreenhouseVagas(boardsCSV, keywords, country string) ([]domain.Vaga, error) {
	boards := parseCSV(boardsCSV)
	if len(boards) == 0 {
		return nil, nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	var vagas []domain.Vaga

	for _, board := range boards {
		endpoint := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true", board)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var payload greenhouseJobsResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, job := range payload.Jobs {
			if !matchesKeywords(job.Title, job.Content, keywords) {
				continue
			}
			if !matchesCountry(job.Location.Name+" "+job.Content, country) {
				continue
			}

			vagas = append(vagas, domain.Vaga{
				Titulo:    normalizeSpace(job.Title),
				Empresa:   companyFromBoard(board),
				URL:       job.AbsoluteURL,
				Descricao: normalizeSpace(job.Content),
				Status:    domain.StatusPendente,
			})
		}
	}

	return vagas, nil
}

type greenhouseJobsResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	Title       string `json:"title"`
	AbsoluteURL string `json:"absolute_url"`
	Content     string `json:"content"`
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
}

// FetchLeverVagas consulta a API pública da Lever por company slug.
// Usa LEVER_COMPANIES em formato CSV com os slugs das empresas.
func FetchLeverVagas(companiesCSV, keywords, country string) ([]domain.Vaga, error) {
	companies := parseCSV(companiesCSV)
	if len(companies) == 0 {
		return nil, nil
	}

	client := &http.Client{Timeout: 20 * time.Second}
	var vagas []domain.Vaga

	for _, company := range companies {
		endpoint := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", company)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var payload []leverPosting
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, job := range payload {
			if !matchesKeywords(job.Text, job.Description, keywords) {
				continue
			}
			if !matchesCountry(job.Categories.Location+" "+job.Description, country) {
				continue
			}

			vagas = append(vagas, domain.Vaga{
				Titulo:    normalizeSpace(job.Text),
				Empresa:   companyFromBoard(company),
				URL:       job.HostedURL,
				Descricao: normalizeSpace(job.Description),
				Status:    domain.StatusPendente,
			})
		}
	}

	return vagas, nil
}

type leverPosting struct {
	Text        string `json:"text"`
	HostedURL   string `json:"hostedUrl"`
	Description string `json:"description"`
	Categories  struct {
		Location string `json:"location"`
	} `json:"categories"`
}

func dedupeVagas(vagas []domain.Vaga) []domain.Vaga {
	if len(vagas) == 0 {
		return vagas
	}

	seen := make(map[string]struct{}, len(vagas))
	result := make([]domain.Vaga, 0, len(vagas))

	for _, v := range vagas {
		key := dedupeKey(v)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, v)
	}

	return result
}

func dedupeKey(v domain.Vaga) string {
	urlKey := strings.ToLower(strings.TrimSpace(v.URL))
	if urlKey != "" {
		return "url:" + urlKey
	}

	title := normalizeSpace(strings.ToLower(v.Titulo))
	company := normalizeSpace(strings.ToLower(v.Empresa))
	if title == "" && company == "" {
		return ""
	}

	return "meta:" + company + "|" + title
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeGupyBoardURL(board string) string {
	u := strings.TrimSpace(board)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	if !strings.Contains(u, "/jobs") {
		u = strings.TrimRight(u, "/") + "/jobs"
	}
	return u
}

func absolutizeURL(baseURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	rel, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(rel).String()
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func matchesKeywords(title, description, keywords string) bool {
	if strings.TrimSpace(keywords) == "" {
		return true
	}

	terms := parseCSV(keywords)
	if len(terms) == 0 {
		terms = []string{keywords}
	}

	if len(terms) == 1 {
		single := strings.TrimSpace(terms[0])
		if strings.Contains(single, " ") {
			for _, piece := range strings.Fields(single) {
				if len(piece) >= 3 {
					terms = append(terms, piece)
				}
			}
		}
	}

	blob := strings.ToLower(title + " " + description)
	for _, t := range terms {
		term := strings.ToLower(strings.TrimSpace(t))
		if term == "" {
			continue
		}
		if strings.Contains(blob, term) {
			return true
		}
	}
	return false
}

func matchesCountry(blob, country string) bool {
	c := strings.ToUpper(strings.TrimSpace(country))
	if c == "" {
		return true
	}

	text := strings.ToLower(blob)
	switch c {
	case "BR", "BRAZIL", "BRASIL":
		return strings.Contains(text, "brasil") || strings.Contains(text, "brazil") || strings.Contains(text, "remoto") || strings.Contains(text, "remote")
	default:
		return true
	}
}

func companyFromURL(rawURL, fallback string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fallback
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	parts := strings.Split(host, ".")
	if len(parts) > 0 && parts[0] != "" {
		return strings.Title(parts[0])
	}
	return fallback
}

func companyFromBoard(board string) string {
	b := strings.TrimSpace(board)
	if b == "" {
		return "greenhouse"
	}
	return strings.Title(strings.ReplaceAll(b, "-", " "))
}
