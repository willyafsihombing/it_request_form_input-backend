package main

import (
	"context"
	"it-request-backend/internal/config"
	"it-request-backend/internal/database"
	onedbsync "it-request-backend/internal/modules/onedb_sync"
	"it-request-backend/internal/routes"
	"it-request-backend/internal/shared/utils"
	"log"

	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println(".env tidak dapat ditemukan, memakai environtment sistem")
	}

	cfg := config.Load()

	utils.CheckJWTSecret()

	database.Connect(cfg.DatabaseURL)
	defer database.DB.Close()

	syncRepo := onedbsync.NewEmployeeSyncRepository(database.DB)
	syncH := onedbsync.NewEmployeeSyncHandler(syncRepo, onedbsync.EmployeeSyncConfigFromEnv())
	syncH.Start(context.Background())

	r := routes.Setup(database.DB, syncH)

	log.Printf("🚀 Server berjalan di port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("❌ Server gagal start:", err)
	}

}
