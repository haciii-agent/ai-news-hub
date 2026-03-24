// Package wechat provides WeChat public account publishing capabilities.
package wechat

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
)

// Client is a WeChat public account API client.
type Client struct {
	appID       string
	secret      string
	accountID   string
	accessToken string
	tokenExpiry time.Time
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewClient creates a new WeChat client from config.
func NewClient(cfg config.WeChatConfig) *Client {
	if !cfg.IsEnabled() {
		return nil
	}
	return &Client{
		appID:      cfg.GetAppID(),
		secret:     cfg.GetSecret(),
		accountID:  cfg.AccountID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Available returns true if the client is configured.
func (c *Client) Available() bool {
	return c != nil && c.appID != "" && c.secret != ""
}

// getAccessToken fetches or returns a cached access token.
func (c *Client) getAccessToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		defer c.mu.RUnlock()
		return c.accessToken, nil
	}
	c.mu.RUnlock()

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		c.appID, c.secret,
	)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn  int    `json:"expires_in"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("access token error %d: %s (IP may not be whitelisted)", result.ErrCode, result.ErrMsg)
	}

	c.mu.Lock()
	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	c.mu.Unlock()

	log.Printf("[wechat] access token refreshed, expires in %ds", result.ExpiresIn)
	return result.AccessToken, nil
}

// apiCall is a generic helper for WeChat API calls.
func (c *Client) apiCall(method, path string, body interface{}) (json.RawMessage, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	var url strings.Builder
	url.WriteString("https://api.weixin.qq.com")
	url.WriteString(path)
	if !strings.Contains(path, "?") {
		url.WriteString("?access_token=")
	} else {
		url.WriteString("&access_token=")
	}
	url.WriteString(token)

	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int            `json:"errcode"`
		ErrMsg  string         `json:"errmsg"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return result.Data, nil
}

// ThumbInfo represents a thumb media ID for a draft article.
type ThumbInfo struct {
	ThumbMediaID string `json:"thumb_media_id"`
	Author       string `json:"author,omitempty"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Digest       string `json:"digest,omitempty"`
	ContentSourceURL string `json:"content_source_url,omitempty"`
	CanComment   int    `json:"can_comment"`
	Comment      int    `json:"comment"`
}

// CreateDraft creates a new article draft in the WeChat draft box.
// Returns the media_id of the created draft.
func (c *Client) CreateDraft(articles []ThumbInfo) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("wechat client not available (check appid/secret)")
	}

	payload := map[string]interface{}{
		"articles": articles,
	}

	data, err := c.apiCall("POST", "/cgi-bin/draft/add", payload)
	if err != nil {
		return "", fmt.Errorf("create draft: %w", err)
	}

	var result struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse draft response: %w", err)
	}
	log.Printf("[wechat] draft created: media_id=%s (%d articles)", result.MediaID, len(articles))
	return result.MediaID, nil
}

// PublishDraft publishes a draft from the draft box to the public account.
func (c *Client) PublishDraft(mediaID string) error {
	if !c.Available() {
		return fmt.Errorf("wechat client not available")
	}

	payload := map[string]string{"media_id": mediaID}
	_, err := c.apiCall("POST", "/cgi-bin/draft/publish", payload)
	if err != nil {
		return fmt.Errorf("publish draft: %w", err)
	}
	log.Printf("[wechat] draft published: media_id=%s", mediaID)
	return nil
}

// PublishArticle creates and publishes a draft in one step.
func (c *Client) PublishArticle(title, author, content, digest, sourceURL string) error {
	mediaID, err := c.CreateDraft([]ThumbInfo{
		{
			ThumbMediaID:     "",
			Author:           author,
			Title:            title,
			Content:          content,
			Digest:           digest,
			ContentSourceURL: sourceURL,
			CanComment:       1,
			Comment:          1,
		},
	})
	if err != nil {
		return err
	}
	return c.PublishDraft(mediaID)
}

// FetchThumbImage uploads an image and returns its media_id for use as article thumbnail.
func (c *Client) FetchThumbImage(imageURL string) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("wechat client not available")
	}

	// Download image
	resp, err := c.httpClient.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	// Upload to WeChat as temporary material (thumb)
	// WeChat requires form-data upload
	body := &bytes.Buffer{}
	writer := multipartWriter{Body: body}
	writer.WriteField("media", imageURL)
	writer.WriteFile("thumb", "image.jpg", resp.Header.Get("Content-Type"), resp.Body)

	token, _ := c.getAccessToken()
	req, err := http.NewRequest("POST",
		"https://api.weixin.qq.com/cgi-bin/media/upload?access_token="+token+"&type=thumb",
		body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp2, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload thumb: %w", err)
	}
	defer resp2.Body.Close()

	var result struct {
		MediaID string `json:"thumb_media_id"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse thumb response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("thumb upload error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return result.MediaID, nil
}

// multipartWriter is a minimal helper for form-data uploads.
type multipartWriter struct {
	Body *bytes.Buffer
}

func (m *multipartWriter) WriteField(key, value string) {
	m.Body.WriteString("--BOUNDARY\r\n")
	m.Body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"\r\n\r\n", key))
	m.Body.WriteString(value + "\r\n")
}

func (m *multipartWriter) WriteFile(fieldName, fileName, contentType string, reader io.Reader) {
	m.Body.WriteString("--BOUNDARY\r\n")
	m.Body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, fileName))
	m.Body.WriteString("Content-Type: " + contentType + "\r\n\r\n")
	io.Copy(m.Body, reader)
	m.Body.WriteString("\r\n--BOUNDARY--\r\n")
}

func (m *multipartWriter) FormDataContentType() string {
	return "multipart/form-data; boundary=BOUNDARY"
}
