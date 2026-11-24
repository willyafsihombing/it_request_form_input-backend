package user

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type UserRepository struct {
	DB *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{DB: db}
}

// Cari user by email atau username
func (r *UserRepository) FindByLogin(login string) (*User, error) {
	var user User
	var deletedAt sql.NullTime
	var employeeID sql.NullString

	query := `
        SELECT
            id, email_addrs, user_name, full_name,
            role, department, title_user, password_hash,employee_id,
			must_change_password,deleted_at, created_at, updated_at
        FROM users
        WHERE (email_addrs = $1 OR user_name = $1)
          AND deleted_at IS NULL
        LIMIT 1
    `

	err := r.DB.QueryRow(query, login).Scan(
		&user.ID, &user.EmailAddrs, &user.UserName, &user.FullName,
		&user.Role, &user.Department, &user.TitleUser, &user.PasswordHash, &employeeID,
		&user.MustChangePassword, &deletedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if employeeID.Valid {
		user.EmployeeID = &employeeID.String
	}

	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return &user, nil
}

// Cari user by ID
func (r *UserRepository) FindByID(id string) (*User, error) {
	var user User
	var deletedAt sql.NullTime
	var employeeID sql.NullString

	query := `
        SELECT
            id, email_addrs, user_name, full_name,
            role, department, title_user, password_hash,employee_id,
            must_change_password,deleted_at, created_at, updated_at
        FROM users
        WHERE id = $1 AND deleted_at IS NULL
    `

	err := r.DB.QueryRow(query, id).Scan(
		&user.ID, &user.EmailAddrs, &user.UserName, &user.FullName,
		&user.Role, &user.Department, &user.TitleUser, &user.PasswordHash, &employeeID,
		&user.MustChangePassword, &deletedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if employeeID.Valid {
		user.EmployeeID = &employeeID.String
	}

	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	return &user, nil
}

// Create user baru
func (r *UserRepository) Create(user *User) error {
	now := time.Now()
	var employeeID interface{}
	if user.EmployeeID != nil && *user.EmployeeID != "" {
		employeeID = *user.EmployeeID
	}
	_, err := r.DB.Exec(`
        INSERT INTO users (
            id,employee_id, email_addrs, user_name, full_name,
            role, department, title_user, password_hash,
            created_at, updated_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `,
		uuid.New().String(),
		employeeID,
		user.EmailAddrs, user.UserName, user.FullName,
		user.Role, user.Department, user.TitleUser, user.PasswordHash,
		now, now,
	)
	return err
}

// Soft delete user
func (r *UserRepository) SoftDelete(id string) error {
	_, err := r.DB.Exec(
		"UPDATE users SET deleted_at = $1 WHERE id = $2",
		time.Now(), id,
	)
	return err
}

// Cek email sudah ada
func (r *UserRepository) ExistsByEmail(email string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM users WHERE email_addrs = $1 AND deleted_at IS NULL",
		email,
	).Scan(&count)
	return count > 0, err
}

func (r *UserRepository) UniqueEmail(local, domain string, used map[string]bool) (string, error) {
	variants := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		l := local
		if i > 1 {
			l = fmt.Sprintf("%s%d", local, i)
		}
		variants = append(variants, l+"@"+domain)
	}
	_, takenEmails, err := r.ExistingLoginNames(nil, variants)
	if err != nil {
		return "", err
	}
	for _, e := range variants {
		if !used[e] && !takenEmails[e] {
			used[e] = true
			return e, nil
		}
	}
	return "", fmt.Errorf("tidak ada email yang tersedia untuk %q", local)
}

// Cek username sudah ada
func (r *UserRepository) ExistsByUsername(username string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM users WHERE user_name = $1 AND deleted_at IS NULL",
		username,
	).Scan(&count)
	return count > 0, err
}

func (r *UserRepository) EmailsByEmployeeIDs(ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.DB.Query(`
		SELECT employee_id::text, COALESCE(email_addrs, '')
		FROM users
		WHERE employee_id::text = ANY($1) AND deleted_at IS NULL`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, err
		}
		out[id] = email
	}
	return out, rows.Err()
}
