package repository

import (
	"database/sql"
	"it-request-backend/models"
	"time"

	"github.com/google/uuid"
)

type UserRepository struct {
	DB *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{DB: db}
}

// Cari user by email atau username
func (r *UserRepository) FindByLogin(login string) (*models.User, error) {
	var user models.User
	var deletedAt sql.NullTime
	var employeeID sql.NullString

	query := `
        SELECT
            id, email_addrs, user_name, full_name,
            role, department, title_user, password_hash,employee_id,
            deleted_at, created_at, updated_at
        FROM users
        WHERE (email_addrs = $1 OR user_name = $1)
          AND deleted_at IS NULL
        LIMIT 1
    `

	err := r.DB.QueryRow(query, login).Scan(
		&user.ID, &user.EmailAddrs, &user.UserName, &user.FullName,
		&user.Role, &user.Department, &user.TitleUser, &user.PasswordHash, &employeeID,
		&deletedAt, &user.CreatedAt, &user.UpdatedAt,
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
func (r *UserRepository) FindByID(id string) (*models.User, error) {
	var user models.User
	var deletedAt sql.NullTime
	var employeeID sql.NullString

	query := `
        SELECT
            id, email_addrs, user_name, full_name,
            role, department, title_user, password_hash,employee_id,
            deleted_at, created_at, updated_at
        FROM users
        WHERE id = $1 AND deleted_at IS NULL
    `

	err := r.DB.QueryRow(query, id).Scan(
		&user.ID, &user.EmailAddrs, &user.UserName, &user.FullName,
		&user.Role, &user.Department, &user.TitleUser, &user.PasswordHash, &employeeID,
		&deletedAt, &user.CreatedAt, &user.UpdatedAt,
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
func (r *UserRepository) Create(user *models.User) error {
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
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
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

// Cek username sudah ada
func (r *UserRepository) ExistsByUsername(username string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM users WHERE user_name = $1 AND deleted_at IS NULL",
		username,
	).Scan(&count)
	return count > 0, err
}
