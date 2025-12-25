package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SystemContext holds metadata about the target system for AI processing
type SystemContext struct {
	Family  string
	Distro  string
	Roles   []string
	Tools   []string
	Network []string
	User    string
}

// AIService defines the interface for AI command generation
type AIService interface {
	GenerateCommand(ctx context.Context, query string, sysCtx SystemContext) (string, error)
}

// MockAIService implements AIService with predefined responses
type MockAIService struct{}

// NewMockAIService creates a new mock AI service
func NewMockAIService() *MockAIService {
	return &MockAIService{}
}

// GenerateCommand returns a mocked command based on the query keywords
func (s *MockAIService) GenerateCommand(ctx context.Context, query string, sysCtx SystemContext) (string, error) {
	// Simulate network latency
	time.Sleep(500 * time.Millisecond)

	lowerQuery := strings.ToLower(query)

	if sysCtx.Family == "windows" {
		if strings.Contains(lowerQuery, "create") && strings.Contains(lowerQuery, "user") {
			return "New-LocalUser -Name 'NewUser' -NoPassword", nil
		}
		if strings.Contains(lowerQuery, "delete") && strings.Contains(lowerQuery, "user") {
			return "Remove-LocalUser -Name 'Joe'", nil
		}
		if strings.Contains(lowerQuery, "list") && strings.Contains(lowerQuery, "process") {
			return "Get-Process | Sort-Object CPU -Descending | Select-Object -First 10", nil
		}
		if strings.Contains(lowerQuery, "ip") || strings.Contains(lowerQuery, "network") {
			return "Get-NetIPAddress | Format-Table", nil
		}
	} else {
		// Linux defaults
		if strings.Contains(lowerQuery, "create") && strings.Contains(lowerQuery, "user") {
			return "sudo useradd -m newuser", nil
		}
		if strings.Contains(lowerQuery, "delete") && strings.Contains(lowerQuery, "user") {
			return "sudo userdel -r joe", nil
		}
		if strings.Contains(lowerQuery, "list") && strings.Contains(lowerQuery, "process") {
			return "ps aux --sort=-%cpu | head -n 10", nil
		}
		if strings.Contains(lowerQuery, "ip") || strings.Contains(lowerQuery, "network") {
			return "ip addr show", nil
		}
	}

	// Fallback
	return fmt.Sprintf("# AI could not generate a confident command for: %s", query), nil
}

// GeminiAIService implements AIService using Google's Gemini API
type GeminiAIService struct {
	apiKey string
	client *http.Client
}

// NewGeminiAIService creates a new Gemini AI service
func NewGeminiAIService(apiKey string) *GeminiAIService {
	return &GeminiAIService{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GenerateCommand calls the Gemini API to generate a command
func (s *GeminiAIService) GenerateCommand(ctx context.Context, query string, sysCtx SystemContext) (string, error) {
	// Prioritized list of models to try
	models := []string{
		"gemini-3-flash",   // Priority 1
		"gemini-2.5-flash", // Priority 2
	}

	// Construct a rich system prompt
	contextDesc := fmt.Sprintf("OS: %s (%s). Current User: %s. ", sysCtx.Family, sysCtx.Distro, sysCtx.User)
	if len(sysCtx.Roles) > 0 {
		contextDesc += fmt.Sprintf("Detected Roles: %s. ", strings.Join(sysCtx.Roles, ", "))
	}
	if len(sysCtx.Tools) > 0 {
		contextDesc += fmt.Sprintf("Installed Tools: %s. ", strings.Join(sysCtx.Tools, ", "))
	}
	if len(sysCtx.Network) > 0 {
		contextDesc += fmt.Sprintf("Network Interfaces: %s. ", strings.Join(sysCtx.Network, ", "))
	}

	systemPrompt := fmt.Sprintf("You are an expert command-line assistant. Context: %s"+
		"The user wants to perform a task. Provide ONLY the exact command to execute. "+
		"Do not provide explanations, markdown, ticks, or 'Here is the command'. "+
		"Just the command text ready to run. "+
		"If the request is ambiguous or unsafe, return a comment starting with # explaining why.",
		contextDesc)

	// Clean up query (remove leading ?)
	cleanQuery := strings.TrimPrefix(strings.TrimSpace(query), "?")
	cleanQuery = strings.TrimSpace(cleanQuery)

	prompt := fmt.Sprintf("%s\nUser Request: %s", systemPrompt, cleanQuery)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1, // Even lower temperature
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error

	// Iterate through models
	for _, model := range models {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.apiKey)

		// Retry loop for transient network errors (but fail fast on 429/404 to next model)
		maxRetries := 2
		for i := 0; i < maxRetries; i++ {
			// Create a new request
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return "", fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := s.client.Do(req)
			if err != nil {
				lastErr = err
				// Network error, retry?
				time.Sleep(200 * time.Millisecond)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				// Success! Parse and return
				var result struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					// If decoding fails, it's likely a bad response, try next model?
					// Or might be our struct. Let's assume it's this model acting up.
					lastErr = fmt.Errorf("failed to decode response from %s: %w", model, err)
					break // Break inner retry, try next model
				}
				if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
					lastErr = fmt.Errorf("no content generated from %s", model)
					break // Break inner retry, try next model
				}
				command := result.Candidates[0].Content.Parts[0].Text

				// --- RESPONSE CLEANING ---
				// 1. Remove markdown code blocks
				command = strings.ReplaceAll(command, "```bash", "")
				command = strings.ReplaceAll(command, "```powershell", "")
				command = strings.ReplaceAll(command, "```", "")

				// 2. Remove echoed query if present
				if strings.HasPrefix(strings.TrimSpace(command), "?") {
					command = strings.TrimPrefix(strings.TrimSpace(command), "?")
					command = strings.TrimPrefix(strings.TrimSpace(command), cleanQuery)
				}

				// 3. Remove "Here is the command:" prefixes
				if idx := strings.Index(command, ":"); idx != -1 && idx < 30 {
					prefix := strings.ToLower(command[:idx])
					if strings.Contains(prefix, "command") || strings.Contains(prefix, "try") {
						command = command[idx+1:]
					}
				}

				// Log success with model used
				// fmt.Printf("Successfully used model: %s\n", model) // Optional: Log to stdout or logger if available
				return strings.TrimSpace(command), nil
			}

			// Handle Errors
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusNotFound {
				// 429 Rate Limit or 404 Model Not Found -> Fail immediately to next model
				lastErr = fmt.Errorf("model %s returned status %s", model, resp.Status)
				break // Break inner retry loop, continue outer loop (next model)
			}

			// 5xx or other errors -> maybe retry?
			lastErr = fmt.Errorf("model %s returned status %s", model, resp.Status)
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	return "", fmt.Errorf("all models failed. last error: %w", lastErr)
}
