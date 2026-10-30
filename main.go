package main

import (
	"it-request-backend/config"
	"it-request-backend/database"
	"it-request-backend/routes"
	"it-request-backend/utils"
	"log"
)

func main() {
	cfg := config.Load()

	utils.CheckJWTSecret()

	database.Connect(cfg.DatabaseURL)
	defer database.DB.Close()

	r := routes.Setup(database.DB)

	log.Printf("🚀 Server berjalan di port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal("❌ Server gagal start:", err)
	}

}
