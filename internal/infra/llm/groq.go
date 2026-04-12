package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"

// GroqClient concentra a integração com a API Llama 3 da Groq.
type GroqClient struct {
	apiKey string
	client *http.Client
}

// NewGroqClient inicializa um novo cliente Groq.
func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type groqRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// AnswerFormField consulta a Llama 3 para responder campos complexos de formulário.
func (g *GroqClient) AnswerFormField(question string, profileContext string) (string, error) {
	systemPrompt := `You are an automated assistant helping to fill out a job application form.
You are provided with a specific field question/label, and the candidate's profile context.
Your task is to answer the question as concisely as possible based on the profile.
If it is a yes/no question, answer with only Yes or No.
Do not add any greetings, explanations, or extra punctuation. Only the direct answer.`

	reqBody := groqRequest{
		Model:       "llama3-8b-8192", // Usa o modelo 8B para respostas rápidas e estáveis.
		Temperature: 0.1,              // Temperatura baixa para reduzir variação.
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Candidate Profile Context:\n%s\n\nForm Question:\n%s", profileContext, question)},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal groq request: %w", err)
	}

	req, err := http.NewRequest("POST", groqAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create groq request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute groq request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq api error: %d - %s", resp.StatusCode, string(body))
	}

	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return "", fmt.Errorf("failed to decode groq response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("groq response had no choices")
	}

	return strings.TrimSpace(groqResp.Choices[0].Message.Content), nil
}
