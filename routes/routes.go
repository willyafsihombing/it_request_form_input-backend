package routes

import (
	"database/sql"
	"it-request-backend/handlers"
	"it-request-backend/middleware"
	"it-request-backend/repository"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Setup(db *sql.DB) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Repository
	requestRepo := repository.NewRequestRepository(db)
	userRepo := repository.NewUserRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	// Handler
	requestHandler := handlers.NewRequestHandler(requestRepo)
	authHandler := handlers.NewAuthHandler(userRepo)
	statsHandler := handlers.NewStatsHandler(statsRepo)

	api := r.Group("/api")
	{
		// Auth — public
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(), authHandler.Me)
		}

		// Requests — protected
		requests := api.Group("/requests", middleware.AuthRequired())
		{
			requests.GET("", requestHandler.GetAll)
			requests.POST("", requestHandler.Create)
			requests.PATCH("/:id", requestHandler.UpdateByID)
			requests.DELETE("/:id", requestHandler.DeletedByID)
		}
		// Public GET form ITR
		api.GET("/requests/:id", requestHandler.GetByID)

		// Stats -> public atau protected
		stats := api.Group("/stats", middleware.AuthRequired())
		{
			stats.GET("", middleware.AuthRequired(), statsHandler.GetStats)
		}

	}

	return r
}
