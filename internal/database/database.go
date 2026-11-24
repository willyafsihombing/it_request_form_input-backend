package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Connect(databaseURL string) {
	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal("❌ Gagal connect DB:", err)
	}
	if err = DB.Ping(); err != nil {
		log.Fatal("❌ DB tidak merespon:", err)
	}
	log.Println("✅ Koneksi PostgreSQL berhasil")
}
