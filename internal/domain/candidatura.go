package domain

// CandidaturaForjada represents the generated mock/personalized resume ready for application.
type CandidaturaForjada struct {
	ID            string
	VagaID        string
	CurriculoPath string
	Status        Status // Typically "FORJADO", "APLICADA", or "ERRO"
}

// VagaComCandidatura is an aggregate structure used to hold both the Job and its ready Resume.
type VagaComCandidatura struct {
	Vaga        Vaga
	Candidatura CandidaturaForjada
}
