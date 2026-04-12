// Package domain contains pure business entities for GhostApply.
// Zero external dependencies — this is the innermost Clean Arch ring.
package domain

import "time"

// Status representa o estado de vida de uma Vaga ou Candidatura.
// O uso de string tipada evita o envio de valores arbitrários.
type Status string

const (
	// StatusNova é aplicado quando a vaga é coletada pela primeira vez.
	StatusNova Status = "NOVA"

	// StatusPendente indica que a vaga entrou na fila de análise.
	StatusPendente Status = "PENDENTE"

	// StatusAnalisada indica que o worker Rust já processou a vaga.
	StatusAnalisada Status = "ANALISADA"

	// StatusForjado indica que já existe um currículo PDF gerado.
	StatusForjado Status = "FORJADO"

	// StatusAplicada indica que o filler enviou a candidatura com sucesso.
	StatusAplicada Status = "APLICADA"

	// StatusErro indica falha irrecuperável no filler ou no worker.
	StatusErro Status = "ERRO"

	// StatusRejeitadoPresencial indica que a vaga foi descartada como não remota.
	StatusRejeitadoPresencial Status = "REJEITADO_PRESENCIAL"

	// StatusDescartada indica que a vaga foi filtrada fora do pipeline.
	StatusDescartada Status = "DESCARTADA"
)

// Vaga representa uma vaga capturada de um portal de empregos.
//
// ID deve ser um UUID v4 gerado pelo chamador.
// URL é a chave de idempotência: duplicatas são ignoradas com INSERT OR IGNORE.
type Vaga struct {
	ID        string    `json:"id"`
	Titulo    string    `json:"titulo"`
	Empresa   string    `json:"empresa"`
	URL       string    `json:"url"`
	Descricao string    `json:"descricao"`
	Status    Status    `json:"status"`
	CriadoEm  time.Time `json:"criado_em"`

	// Campos opcionais para outreach (tarefa de automação)
	RecrutadorNome   *string `json:"recrutador_nome"`
	RecrutadorPerfil *string `json:"recrutador_perfil"`
}
