package main

import "testing"

func TestInferSourceFromURL(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://www.linkedin.com/jobs/view/123", "linkedin"},
		{"https://empresa.gupy.io/jobs/123", "gupy"},
		{"https://boards.greenhouse.io/company/jobs/123", "greenhouse"},
		{"https://jobs.lever.co/company/123", "lever"},
		{"https://example.com/careers/123", "other"},
	}

	for _, c := range cases {
		if got := inferSourceFromURL(c.url); got != c.expected {
			t.Fatalf("inferSourceFromURL(%q) = %q, esperado %q", c.url, got, c.expected)
		}
	}
}

func TestParseCreatedAt(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{name: "rfc3339", input: "2026-04-12T10:45:00Z", valid: true},
		{name: "rfc3339nano", input: "2026-04-12T10:45:00.123456Z", valid: true},
		{name: "sqlite-space", input: "2026-04-12 10:45:00", valid: true},
		{name: "sqlite-t", input: "2026-04-12T10:45:00", valid: true},
		{name: "invalid", input: "12/04/2026 10:45", valid: false},
		{name: "empty", input: "", valid: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok := parseCreatedAt(c.input)
			if ok != c.valid {
				t.Fatalf("parseCreatedAt(%q) válido=%v, esperado=%v", c.input, ok, c.valid)
			}
		})
	}
}
