package parser_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Wanjos-eng/GhostApply/internal/parser"
)

func TestClean_RemovesScriptTags(t *testing.T) {
	raw := `<p>Great opportunity</p><script>alert('xss')</script><p>Apply now</p>`
	got, err := parser.Clean(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "script") || strings.Contains(got, "alert") {
		t.Errorf("script content not removed, got: %q", got)
	}
}

func TestClean_RemovesEmails(t *testing.T) {
	raw := `<p>Contact recruiter@company.com for details</p>`
	got, err := parser.Clean(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "@") {
		t.Errorf("email not removed, got: %q", got)
	}
}

func TestClean_RemovesURLs(t *testing.T) {
	raw := `<p>Visit https://phishing.example.com/apply?token=abc for more info</p>`
	got, err := parser.Clean(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "http") || strings.Contains(got, "phishing") {
		t.Errorf("URL not removed, got: %q", got)
	}
}

func TestClean_ReturnsErrorForEmptyResult(t *testing.T) {
	// Input with only HTML tags and scripts — results in empty string after clean
	raw := `<script>evil()</script><style>.x{}</style>`
	_, err := parser.Clean(raw)
	if !errors.Is(err, parser.ErrEmptyResult) {
		t.Errorf("expected ErrEmptyResult, got: %v", err)
	}
}

func TestClean_PlainTextPassesThrough(t *testing.T) {
	raw := `<p>Senior Software Engineer, remote position in Go and Rust. No travel required.</p>`
	got, err := parser.Clean(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty result for valid job description")
	}
}
