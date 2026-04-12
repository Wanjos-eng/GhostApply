package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvIntOrDefault(t *testing.T) {
	_ = os.Unsetenv("TEST_INT_ENV")
	if got := envIntOrDefault("TEST_INT_ENV", 42); got != 42 {
		t.Fatalf("fallback esperado 42, veio %d", got)
	}

	_ = os.Setenv("TEST_INT_ENV", "15")
	if got := envIntOrDefault("TEST_INT_ENV", 42); got != 15 {
		t.Fatalf("valor parseado esperado 15, veio %d", got)
	}

	_ = os.Setenv("TEST_INT_ENV", "bad")
	if got := envIntOrDefault("TEST_INT_ENV", 42); got != 42 {
		t.Fatalf("fallback em valor inválido esperado 42, veio %d", got)
	}

	_ = os.Setenv("TEST_INT_ENV", "0")
	if got := envIntOrDefault("TEST_INT_ENV", 42); got != 42 {
		t.Fatalf("fallback em valor não positivo esperado 42, veio %d", got)
	}

	_ = os.Unsetenv("TEST_INT_ENV")
}

func TestEnsurePrivateFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secret.env")
	if err := os.WriteFile(path, []byte("x=1\n"), 0o644); err != nil {
		t.Fatalf("falha ao criar arquivo temporário: %v", err)
	}

	ensurePrivateFile(path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("falha ao ler stat do arquivo: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Fatalf("permissão esperada 0600, veio %#o", perm)
	}
}
