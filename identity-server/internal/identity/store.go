package identity

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Store provides data access for the Identity Server.
type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// ── Settings ─────────────────────────────────────────────────────

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.GetContext(ctx, &val, `SELECT value FROM idp_settings WHERE key=$1`, key)
	return val, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO idp_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
	return err
}

func (s *Store) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryxContext(ctx, `SELECT key, value FROM idp_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// ── Users ────────────────────────────────────────────────────────

type User struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	DisplayName  string    `db:"display_name" json:"displayName"`
	Active       bool      `db:"active" json:"active"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type UserWithGroups struct {
	User
	Groups []Group `json:"groups"`
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := s.db.GetContext(ctx, &u, `SELECT * FROM idp_users WHERE username=$1`, username); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int) (*User, error) {
	var u User
	if err := s.db.GetContext(ctx, &u, `SELECT * FROM idp_users WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]UserWithGroups, error) {
	var users []User
	if err := s.db.SelectContext(ctx, &users, `SELECT * FROM idp_users ORDER BY id`); err != nil {
		return nil, err
	}
	result := make([]UserWithGroups, len(users))
	for i, u := range users {
		groups, _ := s.GetUserGroups(ctx, u.ID)
		result[i] = UserWithGroups{User: u, Groups: groups}
	}
	return result, nil
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, displayName string) (int, error) {
	var id int
	err := s.db.GetContext(ctx, &id,
		`INSERT INTO idp_users (username, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id`,
		username, passwordHash, displayName)
	return id, err
}

func (s *Store) UpdateUser(ctx context.Context, id int, displayName string, active bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE idp_users SET display_name=$1, active=$2, updated_at=now() WHERE id=$3`,
		displayName, active, id)
	return err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE idp_users SET password_hash=$1, updated_at=now() WHERE id=$2`,
		passwordHash, id)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idp_users WHERE id=$1`, id)
	return err
}

func (s *Store) UserExists(ctx context.Context) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM idp_users`)
	return count > 0, err
}

// ── Groups ───────────────────────────────────────────────────────

type Group struct {
	ID          int            `db:"id" json:"id"`
	Name        string         `db:"name" json:"name"`
	Description string         `db:"description" json:"description"`
	Actions     pq.StringArray `db:"actions" json:"actions"`
	CreatedAt   time.Time      `db:"created_at" json:"createdAt"`
}

func (s *Store) ListGroups(ctx context.Context) ([]Group, error) {
	var groups []Group
	if err := s.db.SelectContext(ctx, &groups, `SELECT * FROM idp_groups ORDER BY id`); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *Store) GetGroup(ctx context.Context, id int) (*Group, error) {
	var g Group
	if err := s.db.GetContext(ctx, &g, `SELECT * FROM idp_groups WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) CreateGroup(ctx context.Context, name, description string, actions []string) (int, error) {
	var id int
	err := s.db.GetContext(ctx, &id,
		`INSERT INTO idp_groups (name, description, actions) VALUES ($1, $2, $3) RETURNING id`,
		name, description, pq.Array(actions))
	return id, err
}

func (s *Store) UpdateGroup(ctx context.Context, id int, name, description string, actions []string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE idp_groups SET name=$1, description=$2, actions=$3 WHERE id=$4`,
		name, description, pq.Array(actions), id)
	return err
}

func (s *Store) DeleteGroup(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idp_groups WHERE id=$1`, id)
	return err
}

// ── User ↔ Group ─────────────────────────────────────────────────

func (s *Store) GetUserGroups(ctx context.Context, userID int) ([]Group, error) {
	var groups []Group
	err := s.db.SelectContext(ctx, &groups,
		`SELECT g.* FROM idp_groups g
		 JOIN idp_user_groups ug ON ug.group_id = g.id
		 WHERE ug.user_id = $1 ORDER BY g.id`, userID)
	return groups, err
}

func (s *Store) SetUserGroups(ctx context.Context, userID int, groupIDs []int) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM idp_user_groups WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, gid := range groupIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO idp_user_groups (user_id, group_id) VALUES ($1, $2)`, userID, gid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetUserActions returns the merged set of actions from all groups the user belongs to.
func (s *Store) GetUserActions(ctx context.Context, userID int) ([]string, error) {
	groups, err := s.GetUserGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var result []string
	for _, g := range groups {
		for _, a := range g.Actions {
			if !seen[a] {
				seen[a] = true
				result = append(result, a)
			}
		}
	}
	return result, nil
}

// ── Refresh Tokens ───────────────────────────────────────────────

type RefreshToken struct {
	ID        string    `db:"id"`
	UserID    int       `db:"user_id"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *Store) SaveRefreshToken(ctx context.Context, id string, userID int, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO idp_refresh_tokens (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, expiresAt)
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, id string) (*RefreshToken, error) {
	var t RefreshToken
	if err := s.db.GetContext(ctx, &t, `SELECT * FROM idp_refresh_tokens WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) DeleteRefreshToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idp_refresh_tokens WHERE id=$1`, id)
	return err
}

func (s *Store) DeleteUserRefreshTokens(ctx context.Context, userID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idp_refresh_tokens WHERE user_id=$1`, userID)
	return err
}

func (s *Store) CleanExpiredTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idp_refresh_tokens WHERE expires_at < now()`)
	return err
}
