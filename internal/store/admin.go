package store

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
