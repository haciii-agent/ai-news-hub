package store

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// AuthStore defines the interface for auth-related storage operations.
type AuthStore interface {
	CreateUser(tx *sql.Tx, username, email, passwordHash string) (*User, error)
	CheckUsernameExists(username string) (bool, error)
	CheckEmailExists(email string) (bool, error)
	GetUserByUsernameOrEmail(login string) (*User, error)
	GetUserByID(id int64) (*User, error)
	UpdateLastSeen(userID int64) error
	UpdatePassword(userID int64, newHash string) error
	UpdateUsername(userID int64, newUsername string) error
	MigrateAnonUser(tx *sql.Tx, anonUserID, newUserID int64) error
	GetAnonUserByToken(token string) (*User, error)
}

// authStore implements AuthStore backed by *sql.DB.
type authStore struct {
	db *sql.DB
}

// NewAuthStore creates an AuthStore.
func NewAuthStore(db *sql.DB) AuthStore {
	return &authStore{db: db}
}

// CreateUser inserts a new registered user.
// A random UUID is generated as a placeholder token to satisfy NOT NULL UNIQUE constraint.
func (s *authStore) CreateUser(tx *sql.Tx, username, email, passwordHash string) (*User, error) {
	placeholderToken := uuid.New().String()

	result, err := tx.Exec(
		`INSERT INTO users (token, username, email, password_hash, role, created_at, last_seen_at)
		 VALUES (?, ?, ?, ?, 'viewer', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		placeholderToken, username, email, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create user: get last insert id: %w", err)
	}

	return &User{
		ID:       id,
		Token:    placeholderToken,
		Username: &username,
		Email:    &email,
		Role:     "viewer",
	}, nil
}

// CheckUsernameExists returns true if the username is already taken.
func (s *authStore) CheckUsernameExists(username string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE username = ?`, username,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check username exists: %w", err)
	}
	return count > 0, nil
}

// CheckEmailExists returns true if the email is already registered.
func (s *authStore) CheckEmailExists(email string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE email = ?`, email,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return count > 0, nil
}

// GetUserByUsernameOrEmail finds a user by username or email.
func (s *authStore) GetUserByUsernameOrEmail(login string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, token, COALESCE(username, NULL), COALESCE(email, NULL),
		        COALESCE(password_hash, ''), COALESCE(role, 'anonymous'),
		        COALESCE(disabled, 0), COALESCE(merged_into, NULL),
		        created_at, last_seen_at
		 FROM users WHERE username = ? OR email = ? LIMIT 1`,
		login, login,
	).Scan(&u.ID, &u.Token, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.Disabled, &u.MergedInto, &u.CreatedAt, &u.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username or email: %w", err)
	}
	return &u, nil
}

// GetUserByID returns a user by ID.
func (s *authStore) GetUserByID(id int64) (*User, error) {
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
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// UpdateLastSeen updates the last_seen_at timestamp for a user.
func (s *authStore) UpdateLastSeen(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_seen_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("update last_seen: %w", err)
	}
	return nil
}

// UpdatePassword updates the password hash for a user.
func (s *authStore) UpdatePassword(userID int64, newHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, newHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// UpdateUsername updates the username for a user.
func (s *authStore) UpdateUsername(userID int64, newUsername string) error {
	_, err := s.db.Exec(`UPDATE users SET username = ? WHERE id = ?`, newUsername, userID)
	if err != nil {
		return fmt.Errorf("update username: %w", err)
	}
	return nil
}

// MigrateAnonUser migrates anonymous user data (bookmarks, read_history, user_profiles) to a new user.
func (s *authStore) MigrateAnonUser(tx *sql.Tx, anonUserID, newUserID int64) error {
	// 1. Migrate bookmarks (INSERT OR IGNORE to keep new user's data on conflict)
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO bookmarks (user_id, article_id, created_at)
		SELECT ?, article_id, MIN(created_at)
		FROM bookmarks WHERE user_id = ?
		GROUP BY article_id`, newUserID, anonUserID)
	if err != nil {
		return fmt.Errorf("migrate bookmarks: %w", err)
	}

	// 2. Migrate read history
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO read_history (user_id, article_id, read_at)
		SELECT ?, article_id, MAX(read_at)
		FROM read_history WHERE user_id = ?
		GROUP BY article_id`, newUserID, anonUserID)
	if err != nil {
		return fmt.Errorf("migrate read_history: %w", err)
	}

	// 3. Merge profile (if new user has no profile, copy from anon)
	tx.Exec(`
		INSERT OR IGNORE INTO user_profiles (user_id, interests, preferred_categories, updated_at)
		SELECT ?, interests, preferred_categories, CURRENT_TIMESTAMP
		FROM user_profiles WHERE user_id = ?`, newUserID, anonUserID)

	// 4. Mark anonymous user as merged
	_, err = tx.Exec(`UPDATE users SET merged_into = ? WHERE id = ?`, newUserID, anonUserID)
	if err != nil {
		return fmt.Errorf("mark anon merged: %w", err)
	}

	return nil
}

// GetAnonUserByToken finds an anonymous user by their token.
func (s *authStore) GetAnonUserByToken(token string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, token, COALESCE(username, NULL), COALESCE(email, NULL),
		        COALESCE(password_hash, ''), COALESCE(role, 'anonymous'),
		        COALESCE(disabled, 0), COALESCE(merged_into, NULL),
		        created_at, last_seen_at
		 FROM users WHERE token = ?`, token,
	).Scan(&u.ID, &u.Token, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.Disabled, &u.MergedInto, &u.CreatedAt, &u.LastSeenAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get anon user by token: %w", err)
	}
	return &u, nil
}
