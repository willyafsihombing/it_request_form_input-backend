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
	r.SetTrustedProxies(nil)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Disposition", "Content-Length", "Content-Type"},
		AllowCredentials: true,
	}))

	// Repository
	requestRepo := repository.NewRequestRepository(db)
	userRepo := repository.NewUserRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	employeeRepo := repository.NewEmployeeRepository(db)

	// Handler
	requestHandler := handlers.NewRequestHandler(requestRepo)
	authHandler := handlers.NewAuthHandler(userRepo, employeeRepo)
	statsHandler := handlers.NewStatsHandler(statsRepo)
	pdfHandler := handlers.NewPDFHandler(requestRepo)
	employeeHandler := handlers.NewEmployeeHandler(employeeRepo)
	accountHandler := handlers.NewAccountHandler(employeeRepo, userRepo)

	api := r.Group("/api")
	{
		// Auth — public
		auth := api.Group("/auth")

		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(), authHandler.Me)
			auth.POST("/change-password", middleware.AuthRequired(), middleware.MustChangeGuard(), authHandler.ChangePassword)
		}

		// Requests — protected (write)
		requests := api.Group("/requests", middleware.AuthRequired(), middleware.MustChangeGuard())
		{
			requests.GET("", requestHandler.GetAll)
			requests.POST("", requestHandler.Create)
			requests.PATCH("/:id", requestHandler.UpdateByID)
			requests.DELETE("/:id", requestHandler.DeletedByID)
		}

		// Requests — public (read) dalam satu group agar tidak konflik
		requestsPublic := api.Group("/requests")
		{
			requestsPublic.GET("/:id", requestHandler.GetByID)
			requestsPublic.GET("/:id/pdf", pdfHandler.GeneratePDF)
		}

		// Employees
		employeesRead := api.Group("/employees", middleware.AuthRequired(), middleware.MustChangeGuard())
		{
			employeesRead.GET("/search", employeeHandler.SearchInScope)
		}

		employeesView := api.Group("/employees", middleware.AuthRequired(), middleware.RequireRole("admin", "staff"), middleware.MustChangeGuard())
		{
			employeesView.GET("/stats", employeeHandler.Stats)
			employeesView.GET("", employeeHandler.GetAll)
			employeesView.GET("/:id", employeeHandler.GetByID)
		}

		employeesAdmin := api.Group("/employees", middleware.AuthRequired(), middleware.RequireRole("admin"), middleware.MustChangeGuard())
		{
			employeesAdmin.POST("/import/excel", employeeHandler.ImportExcel)
			employeesAdmin.GET("/import/template", employeeHandler.ImportTemplate)
			employeesAdmin.POST("/import", employeeHandler.Import)
			employeesAdmin.POST("", employeeHandler.Create)
			employeesAdmin.PATCH("/:id", employeeHandler.Update)
			employeesAdmin.DELETE("/:id", employeeHandler.Delete)

			//Aktivasi login user
			employeesAdmin.GET("accounts/pending", accountHandler.AccountsPending)
			employeesAdmin.POST("accounts/status", accountHandler.AccountStatus)
			employeesAdmin.POST("accounts/pending", accountHandler.CreateAccounts)
			employeesAdmin.GET("/:id	/reset-passowrd", accountHandler.ResetPassword)
		}

		// Stats
		stats := api.Group("/stats", middleware.AuthRequired(), middleware.MustChangeGuard())
		{
			stats.GET("", statsHandler.GetStats)
		}
	}
	return r
}
