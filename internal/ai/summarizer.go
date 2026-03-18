package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-news-hub/config"
	"ai-news-hub/internal/store"
)

// Summarizer generates AI summaries for articles using an OpenAI-compatible LLM API.
type Summarizer struct {
	apiKey       string
	apiBase      string
	model        string
	httpClient   *http.Client
	semaphore    chan struct{} // concurrency limiter
}

// NewSummarizer creates a new Summarizer from AI config.
// Returns nil if AI is not configured (no API key).
func NewSummarizer(cfg config.AIConfig) *Summarizer {
	key := cfg.GetAPIKey()
	if key == "" || cfg.APIBase == "" {
		return nil
	}
	base := strings.TrimRight(cfg.APIBase, "/")
	if cfg.Model == "" {
		cfg.Model = "glm-4-flash"
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &Summarizer{
		apiKey:    key,
		apiBase:   base,
		model:     cfg.Model,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}
}

// Available returns true if the summarizer is properly configured.
func (s *Summarizer) Available() bool {
	return s != nil
}

// chatRequest is the OpenAI-compatible chat completion request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the OpenAI-compatible chat completion response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    interface{} `json:"code"`
	} `json:"error"`
}

const summaryPromptTemplate = `你是一个科技新闻编辑。请根据以下信息生成一段150-300字的中文新闻摘要。
要求：信息准确、重点突出、语言简洁专业、适合信息流展示。

标题：%s
原文摘要：%s
来源：%s

请直接输出摘要文本，不要添加任何前缀或格式。`

// GenerateSummary generates a Chinese summary for a single article.
func (s *Summarizer) GenerateSummary(article store.Article) (string, error) {
	// Acquire semaphore slot
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	// Build prompt
	title := article.Title
	summary := article.Summary
	if summary == "" {
		summary = "(无原始摘要)"
	} else if len(summary) > 500 {
		summary = summary[:500] + "..."
	}
	source := article.Source
	if source == "" {
		source = "未知来源"
	}

	prompt := fmt.Sprintf(summaryPromptTemplate, title, summary, source)

	reqBody := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := s.apiBase + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	// Execute with retry on 429
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, lastErr = s.httpClient.Do(req)
		if lastErr != nil {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			wait := time.Duration(attempt+1) * 3 * time.Second
			log.Printf("[ai] rate limited, waiting %v before retry", wait)
			time.Sleep(wait)
			continue
		}
		break
	}
	if lastErr != nil {
		return "", fmt.Errorf("http request failed after retries: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("API returned no choices")
	}

	result := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if result == "" {
		return "", fmt.Errorf("API returned empty summary")
	}

	return result, nil
}

// GenerateSummaryForArticle generates a summary and stores it in the database.
func (s *Summarizer) GenerateSummaryForArticle(article store.Article, articleStore store.ArticleStore) error {
	summary, err := s.GenerateSummary(article)
	if err != nil {
		return err
	}
	if err := articleStore.UpdateAISummary(article.ID, summary); err != nil {
		return fmt.Errorf("save ai summary: %w", err)
	}
	log.Printf("[ai] generated summary for article %d: %s", article.ID, article.Title[:min(50, len(article.Title))])
	return nil
}

// GenerateSummariesBatch generates summaries for multiple articles concurrently.
// Returns (success, failed) counts.
func (s *Summarizer) GenerateSummariesBatch(articles []store.Article, articleStore store.ArticleStore) (int, int) {
	if len(articles) == 0 {
		return 0, 0
	}

	var wg sync.WaitGroup
	var successCount, failCount int64
	var mu sync.Mutex

	for i := range articles {
		wg.Add(1)
		go func(a store.Article) {
			defer wg.Done()
			err := s.GenerateSummaryForArticle(a, articleStore)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failCount++
				log.Printf("[ai] failed to summarize article %d: %v", a.ID, err)
			} else {
				successCount++
			}
		}(articles[i])
	}

	wg.Wait()
	return int(successCount), int(failCount)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
