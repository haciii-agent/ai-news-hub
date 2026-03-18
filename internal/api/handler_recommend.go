package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"ai-news-hub/internal/ai"
	"ai-news-hub/internal/store"
)

// --- Recommendation Handler ---

// HandleRecommendations returns personalized article recommendations.
// GET /api/v1/recommendations?page=1&per_page=20
// Header: X-User-Token: <uuid>
func (s *Server) HandleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	q := r.URL.Query()
	page := 1
	perPage := 20

	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			page = v
		}
	}
	if pp := q.Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v >= 1 && v <= 100 {
			perPage = v
		}
	}

	userID := s.getUserID(r)

	// If no user token, return hot articles
	if userID == 0 {
		s.serveHotArticles(w, r, page, perPage)
		return
	}

	// Get user profile
	profile, err := s.ProfileStore.GetProfile(userID)
	if err != nil {
		log.Printf("[api] get profile error: %v", err)
		s.serveHotArticles(w, r, page, perPage)
		return
	}

	// If user has no interests yet, return hot articles
	if len(profile.Interests) == 0 && len(profile.PreferredCategories) == 0 {
		// Check if user has any read history
		reads, _, err := s.ProfileStore.GetTotalReadsAndBookmarks(userID)
		if err != nil || reads == 0 {
			s.serveHotArticles(w, r, page, perPage)
			return
		}
	}

	// Get articles from the last 7 days, exclude already read
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	readIDs, err := s.getReadArticleIDs(userID)
	if err != nil {
		log.Printf("[api] get read IDs error: %v", err)
		readIDs = make(map[int64]bool)
	}

	// Get source read counts for diversity scoring
	sourceCounts, err := s.getSourceReadCounts(userID)
	if err != nil {
		log.Printf("[api] get source counts error: %v", err)
		sourceCounts = make(map[string]int)
	}

	// Query articles from last 7 days
	articles, err := s.getArticlesForRecommendation(sevenDaysAgo, readIDs)
	if err != nil {
		log.Printf("[api] get articles for recommendation error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get recommendations")
		return
	}

	// If not enough recent articles, expand to 30 days
	if len(articles) < perPage {
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		moreArticles, err := s.getArticlesForRecommendation(thirtyDaysAgo, readIDs)
		if err == nil {
			// Merge: add articles not already in the list
			existingIDs := make(map[int64]bool)
			for _, a := range articles {
				existingIDs[a.ID] = true
			}
			for _, a := range moreArticles {
				if !existingIDs[a.ID] {
					articles = append(articles, a)
				}
			}
		}
	}

	// Score and rank articles
	recommender := ai.NewRecommender()
	var recommendations []ai.Recommendation

	for _, article := range articles {
		score, reason := recommender.ScoreForUser(article, profile, sourceCounts)
		recommendations = append(recommendations, ai.Recommendation{
			Article: article,
			Score:   score,
			Reason:  reason,
		})
	}

	// Sort by score descending
	sortRecommendations(recommendations)

	// Paginate
	total := len(recommendations)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage

	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	pagedRecs := recommendations[startIdx:endIdx]
	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	// Build response articles with recommendation reason
	type recArticle struct {
		store.Article
		RecScore float64 `json:"rec_score"`
		RecReason string `json:"rec_reason,omitempty"`
	}

	respArticles := make([]recArticle, len(pagedRecs))
	for i, rec := range pagedRecs {
		respArticles[i] = recArticle{
			Article:   rec.Article,
			RecScore:  rec.Score,
			RecReason: rec.Reason,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"articles":    respArticles,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
		"reason":      "基于你的阅读偏好推荐",
	})
}

// serveHotArticles returns articles sorted by importance score (for guests/new users).
func (s *Server) serveHotArticles(w http.ResponseWriter, r *http.Request, page, perPage int) {
	recommender := ai.NewRecommender()

	filter := store.ArticleFilter{
		Page:    1,
		PerPage: 100, // Get a larger set for scoring
		Sort:    "time",
	}

	articles, _, err := s.Store.QueryArticles(filter)
	if err != nil {
		log.Printf("[api] query articles for hot: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get articles")
		return
	}

	// Score articles
	type scored struct {
		article store.Article
		score   float64
	}
	var scoredArticles []scored
	for _, a := range articles {
		score := recommender.ScoreForHot(a)
		scoredArticles = append(scoredArticles, scored{article: a, score: score})
	}

	// Sort by score
	for i := 0; i < len(scoredArticles); i++ {
		for j := i + 1; j < len(scoredArticles); j++ {
			if scoredArticles[j].score > scoredArticles[i].score {
				scoredArticles[i], scoredArticles[j] = scoredArticles[j], scoredArticles[i]
			}
		}
	}

	total := len(scoredArticles)
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage

	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}

	paged := scoredArticles[startIdx:endIdx]

	type recArticle struct {
		store.Article
		RecScore  float64 `json:"rec_score"`
		RecReason string  `json:"rec_reason,omitempty"`
	}

	respArticles := make([]recArticle, len(paged))
	for i, sa := range paged {
		respArticles[i] = recArticle{
			Article:   sa.article,
			RecScore:  sa.score,
			RecReason: "热门推荐",
		}
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"articles":    respArticles,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
		"reason":      "热门文章推荐",
	})
}

// HandleUserProfile returns the user's interest profile.
// GET /api/v1/user/profile
// Header: X-User-Token: <uuid>
func (s *Server) HandleUserProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	profile, err := s.ProfileStore.GetProfile(userID)
	if err != nil {
		log.Printf("[api] get profile error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get user profile")
		return
	}

	reads, bookmarks, err := s.ProfileStore.GetTotalReadsAndBookmarks(userID)
	if err != nil {
		reads, bookmarks = 0, 0
	}

	streak, err := s.ProfileStore.GetReadingStreak(userID)
	if err != nil {
		streak = &store.ReadingStreak{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"interests":            profile.Interests,
		"preferred_categories": profile.PreferredCategories,
		"total_reads":          reads,
		"total_bookmarks":      bookmarks,
		"reading_streak":       streak.CurrentStreak,
		"longest_streak":       streak.LongestStreak,
		"total_reading_days":   streak.TotalReadingDays,
		"updated_at":           profile.UpdatedAt,
	})
}

// HandleUpdateUserProfile allows users to manually update their preference categories.
// PUT /api/v1/user/profile
// Header: X-User-Token: <uuid>
// Body: {"preferred_categories": ["AI/ML", "开源生态"]}
func (s *Server) HandleUpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "only PUT allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	var req struct {
		PreferredCategories []string `json:"preferred_categories"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.ProfileStore.UpdatePreferredCategories(userID, req.PreferredCategories); err != nil {
		log.Printf("[api] update profile error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"updated": true,
	})
}

// HandleReadingStreak returns the user's reading streak data.
// GET /api/v1/user/streak
// Header: X-User-Token: <uuid>
func (s *Server) HandleReadingStreak(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	userID := s.getUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "X-User-Token header is required")
		return
	}

	streak, err := s.ProfileStore.GetReadingStreak(userID)
	if err != nil {
		log.Printf("[api] get streak error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get reading streak")
		return
	}

	writeJSON(w, http.StatusOK, streak)
}

// --- Helper functions ---

func (s *Server) getReadArticleIDs(userID int64) (map[int64]bool, error) {
	ids := make(map[int64]bool)
	rows, err := s.DB.Query(`SELECT article_id FROM read_history WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func (s *Server) getSourceReadCounts(userID int64) (map[string]int, error) {
	counts := make(map[string]int)
	rows, err := s.DB.Query(`
		SELECT a.source, COUNT(*) as cnt
		FROM read_history rh
		JOIN articles a ON rh.article_id = a.id
		WHERE rh.user_id = ?
		GROUP BY a.source
		ORDER BY cnt DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		counts[source] = count
	}
	return counts, rows.Err()
}

func (s *Server) getArticlesForRecommendation(sinceDate string, excludeIDs map[int64]bool) ([]store.Article, error) {
	// Get articles published/collected since the given date
	query := `
		SELECT id, title, url, source, COALESCE(source_url,''), category,
		       COALESCE(summary,''), COALESCE(content_html,''), COALESCE(image_url,''),
		       published_at, collected_at, language, COALESCE(ai_summary,''),
		       COALESCE(importance_score,0), summary_generated_at
		FROM articles
		WHERE (date(published_at) >= date(?) OR date(collected_at) >= date(?))
		ORDER BY collected_at DESC
		LIMIT 500
	`

	rows, err := s.DB.Query(query, sinceDate, sinceDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []store.Article
	for rows.Next() {
		var a store.Article
		if err := rows.Scan(
			&a.ID, &a.Title, &a.URL, &a.Source, &a.SourceURL,
			&a.Category, &a.Summary, &a.ContentHTML, &a.ImageURL,
			&a.PublishedAt, &a.CollectedAt, &a.Language,
			&a.AISummary, &a.ImportanceScore, &a.SummaryGeneratedAt,
		); err != nil {
			return nil, err
		}

		// Exclude already read articles
		if !excludeIDs[a.ID] {
			articles = append(articles, a)
		}
	}
	return articles, rows.Err()
}

func sortRecommendations(recs []ai.Recommendation) {
	// Simple bubble sort (sufficient for recommendation lists)
	n := len(recs)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if recs[j].Score > recs[i].Score {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}

// Ensure HandleRecommendations can access the database directly.
// This is already available via s.DB in the Server struct.
// We need to make sure ProfileStore is accessible from Server.

// initProfileStore is called in NewServer to initialize the ProfileStore.
func (s *Server) initProfileStore(db *sql.DB) {
	s.ProfileStore = store.NewProfileStore(db)
}
