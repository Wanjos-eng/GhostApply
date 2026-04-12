package main

import "testing"

func TestComposeIMAPAddr(t *testing.T) {
	cases := []struct {
		name     string
		server   string
		port     string
		expected string
	}{
		{name: "server with port", server: "imap.gmail.com:993", port: "143", expected: "imap.gmail.com:993"},
		{name: "server without port", server: "imap.gmail.com", port: "993", expected: "imap.gmail.com:993"},
		{name: "server empty fallback", server: "", port: "", expected: "imap.gmail.com:993"},
		{name: "trimmed values", server: " imap.example.com ", port: " 1993 ", expected: "imap.example.com:1993"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := composeIMAPAddr(c.server, c.port); got != c.expected {
				t.Fatalf("composeIMAPAddr(%q, %q) = %q, esperado %q", c.server, c.port, got, c.expected)
			}
		})
	}
}
