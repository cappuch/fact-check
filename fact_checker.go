package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type FactCheckResult struct {
	Summary string   `json:"summary"`
	Sources []Source `json:"sources"`
}

type ExaSearchResult struct {
	Results []ExaResult `json:"results"`
}

type ExaResult struct {
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Highlights []string `json:"highlights"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Message struct {
	Content string `json:"content"`
}

func PerformFactCheck(query string) FactCheckResult {
	fmt.Printf("QUERY | %s\n", query)

	exaAPIKey := os.Getenv("EXA_API_KEY")
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	openaiBaseURL := os.Getenv("OPENAI_BASE_URL")
	openaiModel := os.Getenv("OPENAI_MODEL_ID")

	fmt.Printf("EXA_API_KEY: %s\n", exaAPIKey)
	fmt.Printf("OPENAI_API_KEY: %s\n", openaiAPIKey)
	fmt.Printf("OPENAI_BASE_URL: %s\n", openaiBaseURL)
	fmt.Printf("OPENAI_MODEL_ID: %s\n", openaiModel)

	if exaAPIKey == "" || openaiAPIKey == "" {
		return FactCheckResult{
			Summary: "API keys for Exa or OpenAI are missing in the environment. Please check your .env file.",
			Sources: []Source{},
		}
	}

	searchResults, err := searchWithExa(query, exaAPIKey)
	if err != nil {
		return FactCheckResult{
			Summary: fmt.Sprintf("Failed to perform search with Exa API: %v", err),
			Sources: []Source{},
		}
	}

	if len(searchResults) == 0 {
		return FactCheckResult{
			Summary: "Could not find any relevant information to fact-check this query.",
			Sources: []Source{},
		}
	}

	var sources []Source
	var contextText strings.Builder

	for i, res := range searchResults {
		highlightText := "No highlights available."
		if len(res.Highlights) > 0 {
			highlightText = strings.Join(res.Highlights, "\n")
		}
		contextText.WriteString(fmt.Sprintf("Source %d: %s (URL: %s)\nHighlights:\n%s\n\n", i+1, res.Title, res.URL, highlightText))
		sources = append(sources, Source{Title: res.Title, URL: res.URL})
	}

	summary, err := generateSummary(query, contextText.String(), openaiAPIKey, openaiBaseURL, openaiModel)
	if err != nil {
		return FactCheckResult{
			Summary: "Failed to generate summary.",
			Sources: sources,
		}
	}

	return FactCheckResult{
		Summary: summary,
		Sources: sources,
	}
}

func searchWithExa(query, apiKey string) ([]ExaResult, error) {
	reqBody := map[string]interface{}{
		"query":       query,
		"type":        "deep",
		"num_results": 10,
		"highlights":  map[string]int{"max_characters": 4000},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.exa.ai/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Exa API error: %s - %s", resp.Status, string(body))
	}

	var result ExaSearchResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Exa response: %v - body: %s", err, string(body))
	}

	return result.Results, nil
}

func generateSummary(query, context, apiKey, baseURL, model string) (string, error) {
	systemPrompt := "You are a professional fact-checking assistant. Your job is to analyze the provided search " +
		"results and determine the accuracy of the user's query. Provide a clear, unbiased summary " +
		"explaining whether the claim is true, false, mixed, or unverifiable. Base your analysis completely " +
		"on the provided context, and cite the sources when making your points (e.g., 'According to Source 1...')."

	userPrompt := fmt.Sprintf("Query to fact-check: %s\n\nContext:\n%s", query, context)

	messages := []OpenAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	openAIReq := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3,
	}

	jsonBody, err := json.Marshal(openAIReq)
	if err != nil {
		return "", err
	}

	var fullURL string
	if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v1/") {
		fullURL = strings.TrimRight(baseURL, "/") + "/chat/completions"
	} else {
		fullURL = strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	}
	fmt.Printf("Full OpenAI URL: %s\n", fullURL)

	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Printf("OpenAI response status: %s\n", resp.Status)
	fmt.Printf("OpenAI response body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error: %s - %s", resp.Status, string(body))
	}

	var result OpenAIResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %v - body: %s", err, string(body))
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}
