package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// CohereClient concentra a integração com a API do Cohere.
type CohereClient struct {
	apiKey string
	client *http.Client
}

// NewCohereClient carrega o cliente com as credenciais do .env.
func NewCohereClient() *CohereClient {
	return &CohereClient{
		apiKey: os.Getenv("COHERE_API_KEY"),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type cohereChatRequest struct {
	Message          string  `json:"message"`
	Model            string  `json:"model"`
	Temperature      float64 `json:"temperature"`
	PreambleOverride string  `json:"preamble_override"`
}

type cohereChatResponse struct {
	Text string `json:"text"`
}

// ClassifyEmail classifica o corpo de um email como REJEICAO, ENTREVISTA ou OUTRO.
// Tarefa 53
func (c *CohereClient) ClassifyEmail(body string) (string, error) {
	if c.apiKey == "" {
		return "OUTRO", fmt.Errorf("COHERE_API_KEY not defined")
	}

	preamble := `You are an automated email classifier for job applications. 
Analyse the provided email body. Is it a job rejection ("REJEICAO") or an interview invitation/progression ("ENTREVISTA")?
If it's neither, classify as "OUTRO".
Respond directly with ONLY ONE of these three strict labels: "REJEICAO", "ENTREVISTA" or "OUTRO". Do not add any punctuation.`

	reqBody := cohereChatRequest{
		Model:            "command-r", // Light, fast instruction-based model 
		Message:          "Email Body:\n\n" + body,
		PreambleOverride: preamble,
		Temperature:      0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "OUTRO", err
	}

	req, err := http.NewRequest("POST", "https://api.cohere.ai/v1/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return "OUTRO", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "OUTRO", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rawResp, _ := io.ReadAll(resp.Body)
		return "OUTRO", fmt.Errorf("Cohere API failure: %d - %s", resp.StatusCode, string(rawResp))
	}

	var parsed cohereChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "OUTRO", err
	}

	// O Cohere devolve exatamente a palavra classificada.
	return parsed.Text, nil
}

// GenerateOutreachMessage monta uma mensagem curta para contato direto no LinkedIn.
// Tarefa 56: contornar o ATS via mensagem direta.
func (c *CohereClient) GenerateOutreachMessage(recruiterName, roleName string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("COHERE_API_KEY not defined")
	}

	if recruiterName == "" {
		recruiterName = "Recrutador(a)"
	}
	if roleName == "" {
		roleName = "vaga na sua equipe"
	}

	preamble := `You are an expert tech recruiter and networking coach. 
Your goal is to generate a short, high-converting LinkedIn direct message for a candidate bypassing the ATS.
The candidate is an Engineering Student focused on Software Architecture, currently building local high-performance systems with Go, Rust, Tauri, and Wails.
Keep it strictly under 500 characters. Tone: confident, concise, and professional.
Output ONLY the message body, no placeholders, no quotes.`

	messagePrompt := fmt.Sprintf("Write an outreach message addressed to '%s' regarding the '%s' role.", recruiterName, roleName)

	reqBody := cohereChatRequest{
		Model:            "command-r",
		Message:          messagePrompt,
		PreambleOverride: preamble,
		Temperature:      0.4, // slight creativity for natural text
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.cohere.ai/v1/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Cohere API failure: %d", resp.StatusCode)
	}

	var parsed cohereChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}

	return parsed.Text, nil
}
