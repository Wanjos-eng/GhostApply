package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestNormalizeDatabasePath(t *testing.T) {
	appDir := "/tmp/ghostapply"

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty uses default", input: "", want: "/tmp/ghostapply/forja_ghost.sqlite"},
		{name: "legacy file uri with pragma", input: "file:forja_ghost.sqlite?_pragma=key('x')", want: "/tmp/ghostapply/forja_ghost.sqlite"},
		{name: "quoted relative path", input: "'db/custom.sqlite'", want: "/tmp/ghostapply/db/custom.sqlite"},
		{name: "memory preserved", input: ":memory:", want: ":memory:"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeDatabasePath(c.input, appDir)
			if got != c.want {
				t.Fatalf("normalizeDatabasePath(%q)=%q want=%q", c.input, got, c.want)
			}
		})
	}
}

func TestBuildDashboardSQLiteDSN(t *testing.T) {
	t.Run("windows path with key", func(t *testing.T) {
		dsn := buildDashboardSQLiteDSN(`C:\Program Files (x86)\GhostApply\forja_ghost.sqlite`, "abc123")
		if !strings.HasPrefix(dsn, "file:C:/Program Files (x86)/GhostApply/forja_ghost.sqlite?") {
			t.Fatalf("dsn prefix inesperado: %q", dsn)
		}
		if !strings.Contains(dsn, "_pragma=key('abc123')") {
			t.Fatalf("dsn sem key pragma: %q", dsn)
		}
	})

	t.Run("key escaping", func(t *testing.T) {
		dsn := buildDashboardSQLiteDSN("/tmp/forja_ghost.sqlite", "te'st")
		if !strings.Contains(dsn, "_pragma=key('te''st')") {
			t.Fatalf("escape de key inválido: %q", dsn)
		}
	})

	t.Run("without key", func(t *testing.T) {
		dsn := buildDashboardSQLiteDSN("/tmp/forja_ghost.sqlite", "")
		if strings.Contains(dsn, "_pragma=key(") {
			t.Fatalf("dsn sem key deveria omitir pragma key: %q", dsn)
		}
	})
}

func TestResolvePreferredDatabasePathUsesLegacyWhenDefaultMissing(t *testing.T) {
	home := t.TempDir()
	appDir := filepath.Join(home, "GhostApply")
	legacyDir := filepath.Join(home, ".ghostapply")

	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	legacyDB := filepath.Join(legacyDir, "forja_ghost.sqlite")
	if err := os.WriteFile(legacyDB, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	t.Setenv("HOME", home)

	got := resolvePreferredDatabasePath("", appDir)
	if got != legacyDB {
		t.Fatalf("expected legacy db path %q, got %q", legacyDB, got)
	}
}

func TestMirrorSQLiteArtifactsCopiesMainAndWalShm(t *testing.T) {
	tmpDir := t.TempDir()
	srcBase := filepath.Join(tmpDir, "source.sqlite")
	dstBase := filepath.Join(tmpDir, "target.sqlite")

	if err := os.WriteFile(srcBase, []byte("main"), 0o600); err != nil {
		t.Fatalf("write src main: %v", err)
	}
	if err := os.WriteFile(srcBase+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatalf("write src wal: %v", err)
	}
	if err := os.WriteFile(srcBase+"-shm", []byte("shm"), 0o600); err != nil {
		t.Fatalf("write src shm: %v", err)
	}

	mirrorSQLiteArtifacts(srcBase, dstBase)

	for _, suffix := range []string{"", "-wal", "-shm"} {
		payload, err := os.ReadFile(dstBase + suffix)
		if err != nil {
			t.Fatalf("expected copied artifact %q: %v", dstBase+suffix, err)
		}
		if len(payload) == 0 {
			t.Fatalf("copied artifact should not be empty: %q", dstBase+suffix)
		}
	}
}
