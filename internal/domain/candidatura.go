package domain

// CandidaturaForjada representa o currículo gerado e pronto para candidatura.
type CandidaturaForjada struct {
	ID            string
	VagaID        string
	CurriculoPath string
	Status        Status // Typically "FORJADO", "APLICADA", or "ERRO"
}

// VagaComCandidatura agrupa a vaga e a candidatura correspondente.
type VagaComCandidatura struct {
	Vaga        Vaga
	Candidatura CandidaturaForjada
}
