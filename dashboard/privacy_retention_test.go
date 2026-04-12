package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestRunRetentionPolicies(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("falha ao abrir sqlite em memória: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE Vaga_Prospectada (
			id TEXT PRIMARY KEY,
			criado_em TEXT NOT NULL
		);
		CREATE TABLE Candidatura_Forjada (
			id TEXT PRIMARY KEY,
			vaga_id TEXT,
			criado_em TEXT NOT NULL
		);
		CREATE TABLE Email_Recrutador (
			id TEXT PRIMARY KEY,
			email TEXT,
			classificacao TEXT,
			corpo TEXT
		);
	`)
	if err != nil {
		t.Fatalf("falha ao criar schema de teste: %v", err)
	}

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -120).Format(time.RFC3339)
	recent := now.AddDate(0, 0, -10).Format(time.RFC3339)

	_, err = db.Exec(`INSERT INTO Vaga_Prospectada (id, criado_em) VALUES ('vaga-old', ?), ('vaga-new', ?)`, old, recent)
	if err != nil {
		t.Fatalf("falha ao inserir vagas: %v", err)
	}
	_, err = db.Exec(`INSERT INTO Candidatura_Forjada (id, vaga_id, criado_em) VALUES ('cand-old', 'vaga-old', ?), ('cand-new', 'vaga-new', ?)`, old, recent)
	if err != nil {
		t.Fatalf("falha ao inserir candidaturas: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO Email_Recrutador (id, email, classificacao, corpo) VALUES
		('e1', 'a@x.com', 'OUTRO', '1'),
		('e2', 'b@x.com', 'OUTRO', '2'),
		('e3', 'c@x.com', 'OUTRO', '3')
	`)
	if err != nil {
		t.Fatalf("falha ao inserir emails: %v", err)
	}

	if err := runRetentionPolicies(db, now, 90, 2); err != nil {
		t.Fatalf("runRetentionPolicies retornou erro: %v", err)
	}

	assertCount := func(table string, expected int) {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("falha ao contar %s: %v", table, err)
		}
		if count != expected {
			t.Fatalf("%s deveria ter %d registros, tem %d", table, expected, count)
		}
	}

	assertCount("Vaga_Prospectada", 1)
	assertCount("Candidatura_Forjada", 1)
	assertCount("Email_Recrutador", 2)
}
