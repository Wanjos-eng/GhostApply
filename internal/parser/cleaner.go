// Package parser provides text sanitisation utilities for raw HTML scraped from job portals.
//
// # Intent
// Raw job descriptions come with HTML tags, tracking pixels, embedded scripts and
// sometimes adversarial content designed to manipulate LLMs (prompt injection).
// This package aggressively strips everything that is not plain human-readable text.
//
// # Constraint (SecOps — Task 24)
// ALL of these must be removed before the text reaches the AI pipeline:
//   - <script> and <style> blocks (code execution vectors)
//   - Email addresses (PII leakage + prompt injection bait)
//   - Hyperlinks and bare URLs (redirect/phishing vectors)
//   - Residual HTML tags
package parser

import (
	"errors"
	"regexp"
	"strings"
)

// ── compiled regexes (package-level = compiled once) ─────────────────────────

var (
	// Removes <script>...</script> blocks including multiline content.
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

	// Removes <style>...</style> blocks.
	reStyle = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)

	// Removes all remaining HTML tags.
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)

	// Removes email addresses — PII and common prompt injection anchor.
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// Removes URLs (http/https/ftp and bare www. links).
	reURL = regexp.MustCompile(`(?i)(https?://|ftp://|www\.)\S+`)

	// Collapses multiple whitespace characters (including newlines) into a single space.
	reWhitespace = regexp.MustCompile(`\s+`)
)

// ErrEmptyResult is returned when sanitisation leaves no readable text.
// Callers must treat this as a signal to discard the job description entirely.
var ErrEmptyResult = errors.New("parser: sanitised text is empty — raw input had no usable content")

// Clean sanitises raw HTML from a job portal, returning plain text safe for AI consumption.
//
// Removal order matters:
//  1. Script/style blocks first (may contain URLs/emails that would confuse later steps)
//  2. All HTML tags
//  3. Emails and URLs
//  4. Whitespace normalisation
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
