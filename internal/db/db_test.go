package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSQLiteDSNWithoutKey(t *testing.T) {
	dsn := buildSQLiteDSN("/tmp/forja_ghost.sqlite", "")
	if strings.Contains(dsn, "_pragma=key(") {
		t.Fatalf("dsn sem key não deve conter pragma key: %q", dsn)
	}
	if !strings.Contains(dsn, "_pragma=journal_mode(WAL)") {
		t.Fatalf("dsn sem journal_mode WAL: %q", dsn)
	}
}

func TestBuildSQLiteDSNWithEscapedKey(t *testing.T) {
	dsn := buildSQLiteDSN("C:\\Program Files (x86)\\GhostApply\\forja_ghost.sqlite", "te'st")
	if !strings.Contains(dsn, "_pragma=key('te''st')") {
		t.Fatalf("dsn com key escapada inválida: %q", dsn)
	}
	expectedPrefix := "file:" + filepath.ToSlash("C:\\Program Files (x86)\\GhostApply\\forja_ghost.sqlite") + "?"
	if !strings.HasPrefix(dsn, expectedPrefix) {
		t.Fatalf("prefixo do dsn inesperado: %q", dsn)
	}
}
