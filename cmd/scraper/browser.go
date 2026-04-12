// Package main — integração do navegador para o scraper do GhostApply.
//
// # Intenção
// Isola o ciclo de vida do Playwright: abertura do navegador, endurecimento
// contra bot, injeção de cookies e cadência humana. Nada aqui toca regra de negócio.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/playwright-community/playwright-go"
)

// NewBrowser abre uma instância do Chromium com endurecimento contra bot.
//
// # Anti-Bot (Tarefa 16)
// --disable-blink-features=AutomationControlled oculta o `navigator.webdriver`.
// Serviços de detecção de bot costumam usar esse sinal.
func NewBrowser(pw *playwright.Playwright) (playwright.Browser, error) {
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true), // Tarefa 15: executa sem janela visível.
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-infobars",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-dev-shm-usage",
			"--window-size=1920,1080",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("NewBrowser: failed to launch Chromium: %w", err)
	}
	return browser, nil
}

// cookieFile espelha a estrutura de uma entrada session.json exportada pelo Playwright.
type cookieFile struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// LoadCookies lê um session.json do Playwright e injeta os cookies no contexto.
//
// # Intenção (Tarefa 17)
// Reutilizar uma sessão exportada evita cair no login wall e em CAPTCHA.
func LoadCookies(ctx playwright.BrowserContext, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("LoadCookies: cannot read '%s': %w", path, err)
	}

	var raw []cookieFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("LoadCookies: invalid JSON in '%s': %w", path, err)
	}

	cookies := make([]playwright.OptionalCookie, 0, len(raw))
	for _, c := range raw {
		cookie := playwright.OptionalCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   playwright.String(c.Domain),
			Path:     playwright.String(c.Path),
			HttpOnly: playwright.Bool(c.HTTPOnly),
			Secure:   playwright.Bool(c.Secure),
		}
		if c.Expires > 0 {
			exp := c.Expires
			cookie.Expires = &exp
		}
		cookies = append(cookies, cookie)
	}

	if err := ctx.AddCookies(cookies); err != nil {
		return fmt.Errorf("LoadCookies: failed to inject cookies: %w", err)
	}
	return nil
}

// HumanSleep pausa a execução por uma duração aleatória entre 1000ms e 4500ms.
//
// # Anti-Bot (Tarefa 19)
// Padrões de tempo muito uniformes entregam automação.
// Atrasos aleatórios imitam leitura humana e reduzem a chance de rate limit.
func HumanSleep() {
	const (
		minMs = 1000
		maxMs = 4500
	)
	ms := minMs + rand.Intn(maxMs-minMs+1) //nolint:gosec // non-cryptographic: timing noise only
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
