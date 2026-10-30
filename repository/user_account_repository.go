package repository

import (
	"database/sql"
	"errors"
	"it-request-backend/models"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

var ErrAccountDuplicate = errors.New("username atau email sudah dipakai")

type NewAccount struct {
	EmployeeID   string
	Email        string
	UserName     string
	FullName     string
	Role         string
	Department   string
	Title        string
	PasswordHash string
}

func (r *UserRepository) InsertAccount(a NewAccount) error {
	now := time.Now()
	_, err := r.DB.Exec(`
		INSERT INTO users (
			id, employee_id, email_addrs, user_name, full_name,
			role, department, title_user, password_hash,
			must_change_password, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true,$10,$10)`,
		uuid.New().String(), a.EmployeeID, a.Email, a.UserName, a.FullName,
		a.Role, a.Department, a.Title, a.PasswordHash, now)
	if isUniqueViolation(err) {
		return ErrAccountDuplicate
	}
	return err
}

func (r *UserRepository) UpdatePassword(id, hash string, mustChange bool) error {
	_, err := r.DB.Exec(`
		UPDATE users
		SET password_hash = $2, must_change_password = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, id, hash, mustChange)
	return err
}

func (r *UserRepository) AccountsByEmployeeIDs(ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := r.DB.Query(`
		SELECT employee_id, user_name
		FROM users
		WHERE employee_id = ANY($1::uuid[]) AND deleted_at IS NULL`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var empID, username string
		if err := rows.Scan(&empID, &username); err != nil {
			return nil, err
		}
		out[empID] = username
	}
	return out, rows.Err()
}

func (r *UserRepository) ExistingLoginNames(usernames, emails []string) (map[string]bool, map[string]bool, error) {
	takenUsers := map[string]bool{}
	takenEmails := map[string]bool{}
	if len(usernames) == 0 && len(emails) == 0 {
		return takenUsers, takenEmails, nil
	}

	rows, err := r.DB.Query(`
		SELECT lower(user_name), lower(email_addrs)
		FROM users
		WHERE lower(user_name) = ANY($1) OR lower(email_addrs) = ANY($2)`,
		pq.Array(usernames), pq.Array(emails))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var u, e string
		if err := rows.Scan(&u, &e); err != nil {
			return nil, nil, err
		}
		takenUsers[u] = true
		takenEmails[e] = true
	}
	return takenUsers, takenEmails, rows.Err()
}

func (r *UserRepository) FindByEmployeeID(employeeID string) (*models.User, error) {
	var u models.User
	err := r.DB.QueryRow(`
		SELECT id, user_name, full_name, role
		FROM users
		WHERE employee_id = $1::uuid AND deleted_at IS NULL
		LIMIT 1`, employeeID).Scan(&u.ID, &u.UserName, &u.FullName, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
