package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// AdminStore defines the interface for admin-related storage operations.
// Implemented by Worker-B.
type AdminStore interface {
	ListUsers(page, perPage int, search string) ([]User, int, error)
	GetUserDetail(id int64) (*UserDetail, error)
	UpdateUserRole(id int64, role string) error
	UpdateUserStatus(id int64, disabled bool) error
	CountUsersByRole(role string) (int, error)
	CountTotalUsers() (int, error)
}

// UserDetail holds extended user info for admin views.
type UserDetail struct {
	User
	TotalBookmarks int          `json:"total_bookmarks"`
	TotalReads     int          `json:"total_reads"`
	Profile        *UserProfile `json:"profile,omitempty"`
}

// adminStore implements AdminStore backed by *sql.DB.
type adminStore struct {
	db *sql.DB
}

// NewAdminStore creates an AdminStore.
func NewAdminStore(db *sql.DB) AdminStore {
	return &adminStore{db: db}
}

// ListUsers returns a paginated list of users, optionally filtered by search keyword.
func (s *adminStore) ListUsers(page, perPage int, search string) ([]User, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	where := "WHERE (merged_into IS NULL OR merged_into = 0)"
	args := []interface{}{}

	if search != "" {
		keyword := "%" + search + "%"
		where += " AND (username LIKE ? OR email LIKE ?)"
		args = append(args, keyword, keyword)
	}

	// Total count
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users `+where, args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Paginated results
	offset := (page - 1) * perPage
	query := `SELECT id, token, COALESCE(username, NULL), COALESCE(email, NULL),
	          COALESCE(password_hash, ''), COALESCE(role, 'anonymous'),
	          COALESCE(disabled, 0), COALESCE(merged_into, NULL),
	          created_at, last_seen_at
	          FROM users ` + where + `
	          ORDER BY id DESC
	          LIMIT ? OFFSET ?`
	queryArgs := append(args, perPage, offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID, &u.Token, &u.Username, &u.Email, &u.PasswordHash,
			&u.Role, &u.Disabled, &u.MergedInto, &u.CreatedAt, &u.LastSeenAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
}

// GetUserDetail returns extended user info including bookmark/read counts and profile.
func (s *adminStore) GetUserDetail(id int64) (*UserDetail, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, token, COALESCE(username, NULL), COALESCE(email, NULL),
		        COALESCE(password_hash, ''), COALESCE(role, 'anonymous'),
		        COALESCE(disabled, 0), COALESCE(merged_into, NULL),
		        created_at, last_seen_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Token, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.Disabled, &u.MergedInto, &u.CreatedAt, &u.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user detail: %w", err)
	}

	// Count bookmarks
	var totalBookmarks int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE user_id = ?`, id).Scan(&totalBookmarks)

	// Count reads
	var totalReads int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM read_history WHERE user_id = ?`, id).Scan(&totalReads)

	// Get profile
	profile, _ := (&profileStore{db: s.db}).GetProfile(id)

	return &UserDetail{
		User:           u,
		TotalBookmarks: totalBookmarks,
		TotalReads:     totalReads,
		Profile:        profile,
	}, nil
}

// UpdateUserRole changes a user's role.
func (s *adminStore) UpdateUserRole(id int64, role string) error {
	role = strings.TrimSpace(role)
	validRoles := map[string]bool{"viewer": true, "editor": true, "admin": true}
	if !validRoles[role] {
		return fmt.Errorf("invalid role: %s", role)
	}

	_, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	return nil
}

// UpdateUserStatus enables or disables a user account.
func (s *adminStore) UpdateUserStatus(id int64, disabled bool) error {
	var val int
	if disabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE users SET disabled = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
}

// CountUsersByRole returns the number of users with the given role.
func (s *adminStore) CountUsersByRole(role string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE role = ? AND (merged_into IS NULL OR merged_into = 0)`,
		role,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return count, nil
}

// CountTotalUsers returns the total number of active (non-merged) users.
func (s *adminStore) CountTotalUsers() (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE merged_into IS NULL OR merged_into = 0`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count total users: %w", err)
	}
	return count, nil
}
