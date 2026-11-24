package routes

import (
	"context"
	"database/sql"
	"log"

	"it-request-backend/internal/middleware"
	"it-request-backend/internal/modules/account"
	"it-request-backend/internal/modules/approve"
	"it-request-backend/internal/modules/auth"
	"it-request-backend/internal/modules/dashboard"
	"it-request-backend/internal/modules/employee"
	"it-request-backend/internal/modules/itrequest"
	"it-request-backend/internal/modules/notify"
	onedbsync "it-request-backend/internal/modules/onedb_sync"
	"it-request-backend/internal/modules/user"
	"it-request-backend/internal/shared/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Setup(db *sql.DB, syncH *onedbsync.EmployeeSyncHandler) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3030", "http://host.docker.internal:3030"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS", "PUT"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Cache-Control", "Pragma"},
		ExposeHeaders:    []string{"Content-Disposition", "Content-Length", "Content-Type"},
		AllowCredentials: true,
	}))

	// Repository
	requestRepo := itrequest.NewRequestRepository(db)
	userRepo := user.NewUserRepository(db)
	statsRepo := dashboard.NewStatsRepository(db)
	employeeRepo := employee.NewEmployeeRepository(db)

	approvalChainRepo := approve.NewChainRepository(db)
	approvalRouteRepo := approve.NewRouteRepository(db)
	approvalActionRepo, err := approve.NewActionRepository(db, approve.SignatureMap{
		approve.StepITCheck:     {Name: "sig_it_admin"},
		approve.StepITManager:   {Name: "sig_msdi_mgr"},
		approve.StepSuptDept:    {Name: "sig_spt_dept"},
		approve.StepDeptManager: {Name: "sig_dept_mgr"},
		approve.StepSrManager:   {Name: "sig_sr_mgr"},
		approve.StepHOO:         {Name: "sig_hoo"},
	})
	if err != nil {
		log.Fatalf("Approval %v", err)
	}
	notifyCfg := notify.ConfigFromEnv()
	mailer, err := notify.NewMailer(notifyCfg)
	if err != nil {
		log.Fatalf("Mailer: %v", err)
	}
	notifyRepo := notify.NewRepository(db, notifyCfg, func(code string) string {
		return approve.StepCode(code).Label()
	})
	notifyWorker := notify.NewWorker(notifyRepo, mailer, notifyCfg)
	notifyWorker.Start(context.Background())
	if notifyCfg.Enabled() {
		approvalActionRepo.UseNotifier(notifyRepo)
	}
	notifyHandler := notify.NewHandler(notifyRepo, mailer, notifyWorker, notifyCfg)

	// Handler
	requestHandler := itrequest.NewRequestHandler(requestRepo, userRepo).
		UseApproval(approvalChainRepo, approvalActionRepo)
	pdfHandler := itrequest.NewPDFHandler(requestRepo)
	authHandler := auth.NewAuthHandler(userRepo, employeeRepo)
	statsHandler := dashboard.NewStatsHandler(statsRepo, requestHandler.ResolveScope)
	employeeHandler := employee.NewEmployeeHandler(employeeRepo)
	accountHandler := account.NewAccountHandler(employeeRepo, userRepo)
	approveHandler := approve.NewHandler(approvalActionRepo, approvalRouteRepo, approvalChainRepo,
		func(c *gin.Context) string {
			if cl := utils.CurrentClaims(c); cl != nil {
				return cl.ID
			}
			return ""
		})
	sigHandler := approve.NewSignatureHandler(approvalActionRepo)

	authed := func(extra ...gin.HandlerFunc) []gin.HandlerFunc {
		base := []gin.HandlerFunc{
			middleware.AuthRequired(),
			middleware.SessionGuard(userRepo),
			middleware.MustChangeGuard(),
		}
		return append(base, extra...)
	}

	api := r.Group("/api")
	{
		// Auth
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.GET("/me", middleware.AuthRequired(), authHandler.Me)
			authGroup.POST("/change-password", append(authed(), authHandler.ChangePassword)...)
		}

		// Requests — protected
		requests := api.Group("/requests", authed()...)
		{
			requests.GET("", requestHandler.GetAll)
			requests.POST("", sigHandler.SignAfter, requestHandler.Create)
			requests.PATCH("/:id", requestHandler.UpdateByID)
			requests.DELETE("/:id", requestHandler.DeletedByID)
			requests.PUT("/:id/resubmit", sigHandler.SignAfter, requestHandler.Resubmit)

			requests.POST("/:id/approve", sigHandler.SignAfter, approveHandler.Approve)
			requests.POST("/:id/reject", approveHandler.Reject)
			requests.GET("/:id/approvals", approveHandler.Progress)
			requests.GET("/:id/signatures", sigHandler.ForRequest)

		}

		// Requests — public (read)
		requestsPublic := api.Group("/requests")
		{
			requestsPublic.GET("/:id", requestHandler.GetByID)
			requestsPublic.GET("/:id/pdf", pdfHandler.GeneratePDF)
			requestsPublic.GET("/:id/print-signatures", sigHandler.ForPrint)
		}

		// Employees — semua user login
		employeesRead := api.Group("/employees", authed()...)
		{
			employeesRead.GET("/search", employeeHandler.SearchInScope)
		}

		// Employees — admin
		employeesAdmin := api.Group("/employees", authed(middleware.RequireRole("admin"))...)
		{
			employeesAdmin.GET("/stats", employeeHandler.Stats)
			employeesAdmin.GET("", employeeHandler.GetAll)
			employeesAdmin.GET("/:id", employeeHandler.GetByID)

			employeesAdmin.POST("/sync", syncH.Sync)

			employeesAdmin.POST("/import/excel", employeeHandler.ImportExcel)
			employeesAdmin.GET("/import/template", employeeHandler.ImportTemplate)
			employeesAdmin.POST("/import", employeeHandler.Import)
			employeesAdmin.POST("", employeeHandler.Create)
			employeesAdmin.PATCH("/:id", employeeHandler.Update)
			employeesAdmin.DELETE("/:id", employeeHandler.Delete)

			// Aktivasi akun login
			employeesAdmin.GET("/accounts/pending", accountHandler.AccountsPending)
			employeesAdmin.POST("/accounts/status", accountHandler.AccountStatus)
			employeesAdmin.POST("/accounts", accountHandler.CreateAccounts)
			employeesAdmin.POST("/:id/reset-password", accountHandler.ResetPassword)
		}

		approvals := api.Group("approvals", authed()...)
		{
			approvals.GET("/inbox", approveHandler.Inbox)
			approvals.GET("/inbox/count", approveHandler.InboxCount)
		}

		approvalRoutes := api.Group("/approval-routes", authed(middleware.RequireRole("admin"))...)
		{
			approvalRoutes.GET("", approveHandler.ListRoutes)
			approvalRoutes.GET("/problems", approveHandler.RouteProblems)
			approvalRoutes.GET("/readiness", approveHandler.Readiness)
			approvalRoutes.POST("", approveHandler.CreateRoute)
			approvalRoutes.DELETE("/:id", approveHandler.DeleteRoute)
		}

		adminNotif := api.Group("/admin/notifications", authed(middleware.RequireRole("admin"))...)
		{
			adminNotif.GET("", notifyHandler.List)
			adminNotif.POST("/test", notifyHandler.Test)
			adminNotif.POST("/run", notifyHandler.RunNow)
			adminNotif.POST("/:id/retry", notifyHandler.Retry)
		}

		me := api.Group("/me", authed()...)
		{
			me.GET("/signature", sigHandler.Get)
			me.PUT("/signature", sigHandler.Put)
			me.DELETE("/signature", sigHandler.Delete)
		}

		// Dashboard / Stats
		stats := api.Group("/stats", authed()...)
		{
			stats.GET("", statsHandler.GetStats)

		}

	}
	return r
}
