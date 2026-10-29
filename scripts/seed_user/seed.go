// scripts/seed_user.go
package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("⚠️  .env tidak ditemukan di ../../.env:", err)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("❌ DATABASE_URL kosong, cek lokasi file .env")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("❌ Gagal koneksi ke DB:", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("❌ DB tidak merespon:", err)
	}
	log.Println("✅ Koneksi berhasil")

	users := []struct {
		EmailAddrs string
		UserName   string
		FullName   string
		Role       string
		Department string
		TitleUser  string
		Password   string
	}{
		{
			EmailAddrs: "admin@unggul.com",
			UserName:   "admin",
			FullName:   "Administrator",
			Role:       "admin",
			Department: "IT",
			TitleUser:  "IT Administrator",
			Password:   "Admin@123",
		},
		{
			EmailAddrs: "it@unggul.com",
			UserName:   "itstaff",
			FullName:   "IT Staff",
			Role:       "staff",
			Department: "IT",
			TitleUser:  "IT Staff",
			Password:   "Staff@123",
		},
		{
			EmailAddrs: "willy@unggul.com",
			UserName:   "willy",
			FullName:   "willy sihombing",
			Role:       "user",
			Department: "HR",
			TitleUser:  "HR Engineer",
			Password:   "User@123",
		},
	}

	for _, u := range users {
		// Cek sudah ada
		var count int
		db.QueryRow(
			"SELECT COUNT(*) FROM users WHERE email_addrs = $1",
			u.EmailAddrs,
		).Scan(&count)

		if count > 0 {
			log.Printf("⚠️  User %s sudah ada, skip", u.EmailAddrs)
			continue
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), 12)
		if err != nil {
			log.Printf("❌ Gagal hash password %s: %v", u.EmailAddrs, err)
			continue
		}

		now := time.Now()
		id := uuid.New().String()

		_, err = db.Exec(`
			INSERT INTO users (
				id, email_addrs, user_name, full_name,
				role, department, title_user, password_hash,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`,
			id,
			u.EmailAddrs, u.UserName, u.FullName,
			u.Role, u.Department, u.TitleUser, string(hash),
			now, now,
		)

		if err != nil {
			log.Printf("❌ Gagal insert %s: %v", u.EmailAddrs, err)
			continue
		}

		log.Printf("✅ User created: %s (password: %s)", u.EmailAddrs, u.Password)
	}

	log.Println("🎉 Seed selesai")
}
