package main

import "testing"

func TestNormalizeRuntimeDataPath(t *testing.T) {
	appDir := "/tmp/ghostapply"

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "legacy file uri", input: "file:forja_ghost.sqlite?_pragma=key('x')", want: "/tmp/ghostapply/forja_ghost.sqlite"},
		{name: "quoted relative", input: "'db/custom.sqlite'", want: "/tmp/ghostapply/db/custom.sqlite"},
		{name: "memory preserved", input: ":memory:", want: ":memory:"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeRuntimeDataPath(c.input, appDir)
			if got != c.want {
				t.Fatalf("normalizeRuntimeDataPath(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}
