package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSQLiteDSNWithoutKey(t *testing.T) {
	dsn := buildSQLiteDSN("/tmp/forja_ghost.sqlite")
	if strings.Contains(dsn, "_pragma=key(") {
		t.Fatalf("dsn sem key não deve conter pragma key: %q", dsn)
	}
	if dsn != "file:/tmp/forja_ghost.sqlite" {
		t.Fatalf("dsn inesperado: %q", dsn)
	}
}

func TestBuildSQLiteDSNUsesFileURI(t *testing.T) {
	dsn := buildSQLiteDSN("C:\\Program Files (x86)\\GhostApply\\forja_ghost.sqlite")
	expectedPrefix := "file:" + filepath.ToSlash("C:\\Program Files (x86)\\GhostApply\\forja_ghost.sqlite")
	if !strings.HasPrefix(dsn, expectedPrefix) {
		t.Fatalf("prefixo do dsn inesperado: %q", dsn)
	}
	if strings.Contains(dsn, "_pragma=") || strings.Contains(dsn, "key=") {
		t.Fatalf("dsn não deve depender de pragmas do driver cgo: %q", dsn)
	}
}
