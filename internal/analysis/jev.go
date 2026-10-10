// Package analysis estima qué tan probable es que un reporte sea falso.
// Combina reglas sobre la base de datos con el modelo Jev de TypeSafe AI.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Endpoint y modelo de Jev que usa [NewJevClient].
const (
	defaultJevURL   = "https://api.typesafe.ai/v1/systemone"
	defaultJevModel = "jev-latest"
)

// Question es una pregunta tipada para Jev.
// Type: "noul" (sí/no, 0-1), "choice" (elige una opción) o "score" (niveles 0..n-1).
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Noul arma una pregunta de sí/no. Jev regresa la probabilidad de que sea verdadera.
func Noul(instructions, whenTrue, whenFalse string) Question {
	return Question{
		Type:         "noul",
		Instructions: instructions,
		Criteria:     map[string]string{"true": whenTrue, "false": whenFalse},
	}
}

// Score arma una pregunta con niveles ordenados (de 2 a 10). El nivel 0 es el primero.
func Score(instructions string, levels ...string) Question {
	return Question{Type: "score", Instructions: instructions, Criteria: levels}
}

// Answer es la respuesta de Jev a una pregunta. Solo se llenan los campos de su tipo.
type Answer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

// jevRequest es el cuerpo de POST /v1/systemone.
type jevRequest struct {
	Model     string              `json:"model"`
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// jevResponse es la respuesta de POST /v1/systemone; Answers usa los mismos ids que las preguntas.
type jevResponse struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
}

// JevError es un error HTTP de la API de TypeSafe.
type JevError struct {
	Status int
	Body   string
}

// Error implementa la interfaz error.
func (e *JevError) Error() string {
	return fmt.Sprintf("jev respondió %d: %s", e.Status, e.Body)
}

// Retryable indica si vale la pena reintentar más tarde (límite de uso, saturación o caída).
func (e *JevError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == 529 || e.Status >= 500
}

// IsAccountError indica que el problema es de la cuenta de TypeSafe (clave inválida,
// sin permiso o sin saldo), no del reporte. Reintentar no sirve hasta corregir la cuenta.
func IsAccountError(err error) bool {
	var jerr *JevError
	if errors.As(err, &jerr) {
		return jerr.Status == http.StatusUnauthorized ||
			jerr.Status == http.StatusForbidden ||
			jerr.Status == http.StatusPaymentRequired
	}
	return false
}

// IsRetryable indica si un error de Jev es temporal. Los errores de red también lo son.
func IsRetryable(err error) bool {
	var jerr *JevError
	if errors.As(err, &jerr) {
		return jerr.Retryable()
	}
	return err != nil
}

// JevClient llama a la API de TypeSafe (POST /v1/systemone).
type JevClient struct {
	APIKey string
	URL    string
	Model  string
	HTTP   *http.Client
}

// NewJevClient crea un cliente con los valores por defecto. Regresa nil si no hay API key.
func NewJevClient(apiKey string) *JevClient {
	if apiKey == "" {
		return nil
	}
	return &JevClient{
		APIKey: apiKey,
		URL:    defaultJevURL,
		Model:  defaultJevModel,
		HTTP:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Ask manda el estado y todas las preguntas en una sola petición.
// Jev evalúa las preguntas en paralelo contra el mismo estado.
func (c *JevClient) Ask(ctx context.Context, state any, questions map[string]Question) (map[string]Answer, error) {
	body, err := json.Marshal(jevRequest{Model: c.Model, State: state, Questions: questions})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &JevError{Status: resp.StatusCode, Body: string(raw)}
	}

	var parsed jevResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("respuesta de jev inválida: %w", err)
	}

	for id := range questions {
		if _, ok := parsed.Answers[id]; !ok {
			return nil, fmt.Errorf("jev no respondió la pregunta %q", id)
		}
	}

	return parsed.Answers, nil
}
