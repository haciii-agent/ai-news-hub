package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Comment 评论数据模型
type Comment struct {
	ID        int64  `json:"id"`
	ArticleID int64  `json:"article_id"`
	UserID    int64  `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// InteractionInfo 文章互动信息
type InteractionInfo struct {
	LikesCount    int64 `json:"likes_count"`
	CommentsCount int64 `json:"comments_count"`
	IsLiked       bool  `json:"is_liked,omitempty"`
}

// CommentStore 评论和点赞存储接口
type CommentStore interface {
	AddComment(articleID, userID int64, content string) (*Comment, error)
	DeleteComment(commentID, userID int64) error
	ListComments(articleID int64, page, perPage int) ([]Comment, int, error)

	LikeArticle(articleID, userID int64) error
	UnlikeArticle(articleID, userID int64) error
	IsLiked(articleID, userID int64) (bool, error)
	GetLikesCount(articleID int64) (int64, error)
	GetCommentsCount(articleID int64) (int64, error)
	GetArticleInteractions(articleID, userID int64) (*InteractionInfo, error)
	BatchGetInteractions(articleIDs []int64, userID int64) (map[int64]InteractionInfo, error)
}

// commentStore implements CommentStore backed by a *sql.DB (SQLite).
type commentStore struct {
	db *sql.DB
}

// NewCommentStore wraps an open *sql.DB into a CommentStore.
func NewCommentStore(db *sql.DB) CommentStore {
	return &commentStore{db: db}
}

// AddComment creates a new comment for an article.
func (s *commentStore) AddComment(articleID, userID int64, content string) (*Comment, error) {
	result, err := s.db.Exec(
		`INSERT INTO comments (article_id, user_id, content, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		articleID, userID, content,
	)
	if err != nil {
		return nil, fmt.Errorf("add comment: %w", err)
	}

	id, _ := result.LastInsertId()

	var c Comment
	err = s.db.QueryRow(
		`SELECT id, article_id, user_id, content, created_at FROM comments WHERE id = ?`, id,
	).Scan(&c.ID, &c.ArticleID, &c.UserID, &c.Content, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch new comment: %w", err)
	}

	return &c, nil
}

// DeleteComment deletes a comment if it belongs to the given user.
func (s *commentStore) DeleteComment(commentID, userID int64) error {
	result, err := s.db.Exec(
		`DELETE FROM comments WHERE id = ? AND user_id = ?`,
		commentID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("comment not found or not owned by user")
	}

	return nil
}

// ListComments returns a paginated list of comments for an article (newest first).
func (s *commentStore) ListComments(articleID int64, page, perPage int) ([]Comment, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	// Total count
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM comments WHERE article_id = ?`, articleID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}

	// Paginated results (newest first)
	offset := (page - 1) * perPage
	rows, err := s.db.Query(
		`SELECT id, article_id, user_id, content, created_at FROM comments
		 WHERE article_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		articleID, perPage, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.UserID, &c.Content, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate comments: %w", err)
	}

	return comments, total, nil
}

// LikeArticle adds a like for an article (idempotent).
func (s *commentStore) LikeArticle(articleID, userID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO likes (article_id, user_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(article_id, user_id) DO NOTHING`,
		articleID, userID,
	)
	if err != nil {
		return fmt.Errorf("like article %d for user %d: %w", articleID, userID, err)
	}
	return nil
}

// UnlikeArticle removes a like for an article.
func (s *commentStore) UnlikeArticle(articleID, userID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM likes WHERE article_id = ? AND user_id = ?`,
		articleID, userID,
	)
	if err != nil {
		return fmt.Errorf("unlike article %d for user %d: %w", articleID, userID, err)
	}
	return nil
}

// IsLiked checks if a user has liked a specific article.
func (s *commentStore) IsLiked(articleID, userID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM likes WHERE article_id = ? AND user_id = ?`,
		articleID, userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check like: %w", err)
	}
	return count > 0, nil
}

// GetLikesCount returns the total likes for an article.
func (s *commentStore) GetLikesCount(articleID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM likes WHERE article_id = ?`, articleID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count likes: %w", err)
	}
	return count, nil
}

// GetCommentsCount returns the total comments for an article.
func (s *commentStore) GetCommentsCount(articleID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM comments WHERE article_id = ?`, articleID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count comments: %w", err)
	}
	return count, nil
}

// GetArticleInteractions returns interaction info for a single article.
func (s *commentStore) GetArticleInteractions(articleID, userID int64) (*InteractionInfo, error) {
	info := &InteractionInfo{}

	err := s.db.QueryRow(
		`SELECT
			(SELECT COUNT(*) FROM likes WHERE article_id = ?) as likes_count,
			(SELECT COUNT(*) FROM comments WHERE article_id = ?) as comments_count`,
		articleID, articleID,
	).Scan(&info.LikesCount, &info.CommentsCount)
	if err != nil {
		return nil, fmt.Errorf("get article interactions: %w", err)
	}

	// Check if current user liked (only if userID > 0)
	if userID > 0 {
		liked, err := s.IsLiked(articleID, userID)
		if err == nil {
			info.IsLiked = liked
		}
	}

	return info, nil
}

// BatchGetInteractions returns interaction info for multiple articles at once.
func (s *commentStore) BatchGetInteractions(articleIDs []int64, userID int64) (map[int64]InteractionInfo, error) {
	result := make(map[int64]InteractionInfo, len(articleIDs))
	if len(articleIDs) == 0 {
		return result, nil
	}

	// Build IN clause
	idStrs := make([]string, len(articleIDs))
	for i, id := range articleIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}
	inClause := strings.Join(idStrs, ",")

	// Likes counts
	likesRows, err := s.db.Query(
		`SELECT article_id, COUNT(*) FROM likes WHERE article_id IN (` + inClause + `) GROUP BY article_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("batch get likes: %w", err)
	}
	for likesRows.Next() {
		var articleID int64
		var count int64
		if err := likesRows.Scan(&articleID, &count); err != nil {
			likesRows.Close()
			return nil, fmt.Errorf("scan likes count: %w", err)
		}
		if info, ok := result[articleID]; ok {
			info.LikesCount = count
			result[articleID] = info
		} else {
			result[articleID] = InteractionInfo{LikesCount: count}
		}
	}
	likesRows.Close()

	// Comments counts
	commentsRows, err := s.db.Query(
		`SELECT article_id, COUNT(*) FROM comments WHERE article_id IN (` + inClause + `) GROUP BY article_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("batch get comments: %w", err)
	}
	for commentsRows.Next() {
		var articleID int64
		var count int64
		if err := commentsRows.Scan(&articleID, &count); err != nil {
			commentsRows.Close()
			return nil, fmt.Errorf("scan comments count: %w", err)
		}
		if info, ok := result[articleID]; ok {
			info.CommentsCount = count
			result[articleID] = info
		} else {
			result[articleID] = InteractionInfo{CommentsCount: count}
		}
	}
	commentsRows.Close()

	// Ensure all article IDs are in result (even with 0 counts)
	for _, id := range articleIDs {
		if _, ok := result[id]; !ok {
			result[id] = InteractionInfo{}
		}
	}

	// If user is authenticated, check liked status
	if userID > 0 {
		likedRows, err := s.db.Query(
			`SELECT article_id FROM likes WHERE user_id = ? AND article_id IN (`+inClause+`)`,
			userID,
		)
		if err == nil {
			for likedRows.Next() {
				var articleID int64
				if err := likedRows.Scan(&articleID); err == nil {
					info := result[articleID]
					info.IsLiked = true
					result[articleID] = info
				}
			}
			likedRows.Close()
		}
	}

	return result, nil
}
