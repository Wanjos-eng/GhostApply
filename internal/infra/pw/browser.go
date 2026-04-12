package pw

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/playwright-community/playwright-go"
)

// NewBrowser abre uma instância do Chromium com endurecimento contra bot.
func NewBrowser(pw *playwright.Playwright) (playwright.Browser, error) {
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // Durante depuração, o filler pode rodar com interface visível.
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

// LoadCookies lê um session.json e injeta os cookies no BrowserContext.
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

// HumanSleep pausa a execução por uma duração aleatória.
func HumanSleep() {
	const (
		minMs = 1000
		maxMs = 2500
	)
	ms := minMs + rand.Intn(maxMs-minMs+1)
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// TypeHumanly writes string text into a locator, pausing between 50 and 150ms per keystroke.
func TypeHumanly(loc playwright.Locator, text string) error {
	if err := loc.Fill(""); err != nil {
		return fmt.Errorf("TypeHumanly: failed to clear initial field: %w", err)
	}
	
	// Clica no campo para simular o foco de uma pessoa.
	if err := loc.Click(); err != nil {
		return fmt.Errorf("TypeHumanly: failed to focus field: %w", err)
	}

	for _, char := range text {
		delayMs := 50 + rand.Intn(100) // 50 to 150ms
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		
		err := loc.Type(string(char))
		if err != nil {
			return fmt.Errorf("TypeHumanly: failed while typing char '%c': %w", char, err)
		}
	}
	return nil
}
