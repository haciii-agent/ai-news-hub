package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-news-hub/internal/collector"
)

// HandleArticleContent handles GET /api/v1/articles/{id}/content
// Proxies the original article page, extracts main content via Readability,
// caches it in the DB, and returns the HTML.
func (s *Server) HandleArticleContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	// Extract article ID from path: /api/v1/articles/{id}/content
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/articles/")
	// Remove trailing /content
	idStr := strings.TrimSuffix(path, "/content")
	idStr = strings.TrimSuffix(idStr, "/")

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeError(w, http.StatusBadRequest, "article ID must be a number")
		return
	}

	// Look up article
	article, err := s.Store.GetArticleByID(id)
	if err != nil {
		log.Printf("[api] get article for content: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get article")
		return
	}
	if article == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	// Return cached content if available
	if article.ContentHTML != "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"html":          article.ContentHTML,
			"title":         article.Title,
			"cached":        true,
			"fetch_time_ms": 0,
		})
		return
	}

	// Fetch and extract content
	start := time.Now()
	extracted, err := collector.FetchAndExtract(article.URL, 10*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[api] content extraction failed for article %d (%s): %v", id, article.URL, err)

		// Return 504 for timeout, 200 with empty html for other errors
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "http request") {
			writeJSON(w, http.StatusGatewayTimeout, map[string]interface{}{
				"html":          "",
				"title":         article.Title,
				"cached":        false,
				"fetch_time_ms": elapsed.Milliseconds(),
				"error":         "fetch timeout or network error",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"html":          "",
			"title":         article.Title,
			"cached":        false,
			"fetch_time_ms": elapsed.Milliseconds(),
			"error":         "content extraction failed",
		})
		return
	}

	contentHTML := extracted.HTML
	extractedTitle := extracted.Title

	// Cache to DB (best effort, don't fail the response)
	if contentHTML != "" {
		if cacheErr := s.Store.UpdateContentHTML(id, contentHTML); cacheErr != nil {
			log.Printf("[api] failed to cache content_html for article %d: %v", id, cacheErr)
		}
	}

	// Use extracted title as fallback
	title := article.Title
	if extractedTitle != "" {
		title = extractedTitle
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"html":          contentHTML,
		"title":         title,
		"cached":        false,
		"fetch_time_ms": elapsed.Milliseconds(),
	})
}
