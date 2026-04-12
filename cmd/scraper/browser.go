// Package main — browser wiring for the GhostApply scraper.
//
// # Intent
// Isolate all Playwright browser lifecycle concerns: launching, anti-bot hardening,
// cookie injection and human-like timing. Nothing here touches business logic.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/playwright-community/playwright-go"
)

// NewBrowser launches a Chromium instance with anti-bot hardening applied.
//
// # Anti-Bot (Task 16)
// --disable-blink-features=AutomationControlled hides the `navigator.webdriver`
// property that bot-detection services (Cloudflare, LinkedIn ThrustBuster) check.
func NewBrowser(pw *playwright.Playwright) (playwright.Browser, error) {
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true), // Task 15: invisible to the OS
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

// cookieFile mirrors the structure of a Playwright-exported session.json entry.
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

// LoadCookies reads a Playwright session.json and injects all cookies into ctx.
//
// # Intent (Task 17)
// Reusing an exported browser session avoids triggering LinkedIn's login-wall
// and CAPTCHA flows that fire on fresh sessions from headless browsers.
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

// HumanSleep pauses execution for a random duration between 1000ms and 4500ms.
//
// # Anti-Bot (Task 19)
// Uniform timing patterns are a primary signal for bot-detection ML models.
// Randomised delays mimic human reading/browsing cadence and reduce the
// fingerprint entropy that triggers rate-limiting on LinkedIn.
func HumanSleep() {
	const (
		minMs = 1000
		maxMs = 4500
	)
	ms := minMs + rand.Intn(maxMs-minMs+1) //nolint:gosec // non-cryptographic: timing noise only
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
