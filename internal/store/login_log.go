package store

import (
	"database/sql"
	"fmt"
)

// LoginLog represents a login log entry.
type LoginLog struct {
	ID         int64  `json:"id"`
	UserID     *int64 `json:"user_id,omitempty"`
	Username   string `json:"username"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	Success    bool   `json:"success"`
	FailReason string `json:"fail_reason,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// LoginLogStore defines the interface for login log storage.
type LoginLogStore interface {
	RecordLog(log *LoginLog) error
	GetRecentLogs(userID int64, limit int) ([]LoginLog, error)
}

// loginLogStore implements LoginLogStore backed by *sql.DB.
type loginLogStore struct {
	db *sql.DB
}

// NewLoginLogStore creates a LoginLogStore.
func NewLoginLogStore(db *sql.DB) LoginLogStore {
	return &loginLogStore{db: db}
}

// RecordLog inserts a login log entry.
func (s *loginLogStore) RecordLog(log *LoginLog) error {
	_, err := s.db.Exec(
		`INSERT INTO login_logs (user_id, username, ip_address, user_agent, success, fail_reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		log.UserID, log.Username, log.IPAddress, log.UserAgent, log.Success, log.FailReason,
	)
	if err != nil {
		return fmt.Errorf("record login log: %w", err)
	}
	return nil
}

// GetRecentLogs returns recent login logs for a user.
func (s *loginLogStore) GetRecentLogs(userID int64, limit int) ([]LoginLog, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT id, user_id, username, ip_address, user_agent, success, fail_reason, created_at
		 FROM login_logs WHERE user_id = ?
		 ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent login logs: %w", err)
	}
	defer rows.Close()

	var logs []LoginLog
	for rows.Next() {
		var l LoginLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.IPAddress,
			&l.UserAgent, &l.Success, &l.FailReason, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan login log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
