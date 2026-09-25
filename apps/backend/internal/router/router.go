package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/handler"
	appmiddleware "github.com/rezanii/tracking-cashflow/apps/backend/internal/middleware"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/service"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/validator"
)

// authRateLimit throttles credential endpoints per IP to slow down password guessing.
const (
	authRateLimit  = 10
	authRateWindow = time.Minute
)

// New wires the dependency graph and returns the HTTP handler. Construction happens once at
// startup, so every request reuses the same services and connection pool.
func New(cfg config.Config, db *gorm.DB) http.Handler {
	tokens := utils.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiration)
	requestValidator := validator.New()

	users := repository.NewUserRepository(db)
	categories := repository.NewCategoryRepository(db)
	transactions := repository.NewTransactionRepository(db)
	reports := repository.NewReportRepository(db)
	txManager := repository.NewTxManager(db)

	authService := service.NewAuthService(users, tokens)
	categoryService := service.NewCategoryService(categories)
	transactionService := service.NewTransactionService(transactions, categories, txManager)
	reportService := service.NewReportService(reports, transactions)

	authHandler := handler.NewAuthHandler(authService, requestValidator)
	categoryHandler := handler.NewCategoryHandler(categoryService, requestValidator)
	transactionHandler := handler.NewTransactionHandler(transactionService, requestValidator)
	reportHandler := handler.NewReportHandler(reportService)

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(appmiddleware.RequestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(appmiddleware.SecureHeaders)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		utils.OK(w, "Service is healthy", map[string]string{"status": "ok"})
	})

	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Group(func(public chi.Router) {
				public.Use(httprate.LimitByIP(authRateLimit, authRateWindow))
				public.Post("/register", authHandler.Register)
				public.Post("/login", authHandler.Login)
			})

			auth.Group(func(private chi.Router) {
				private.Use(appmiddleware.Authenticator(tokens))
				private.Post("/logout", authHandler.Logout)
				private.Get("/me", authHandler.Me)
			})
		})

		api.Group(func(private chi.Router) {
			private.Use(appmiddleware.Authenticator(tokens))

			private.Route("/categories", func(categories chi.Router) {
				categories.Get("/", categoryHandler.List)
				categories.Post("/", categoryHandler.Create)
				categories.Get("/{id}", categoryHandler.Get)
				categories.Put("/{id}", categoryHandler.Update)
				categories.Patch("/{id}/status", categoryHandler.SetStatus)
				categories.Delete("/{id}", categoryHandler.Delete)
			})

			private.Route("/transactions", func(transactions chi.Router) {
				transactions.Get("/", transactionHandler.List)
				transactions.Post("/", transactionHandler.Create)
				transactions.Get("/{id}", transactionHandler.Get)
				transactions.Put("/{id}", transactionHandler.Update)
				transactions.Delete("/{id}", transactionHandler.Delete)
			})

			private.Get("/dashboard/summary", reportHandler.Dashboard)

			private.Route("/reports", func(reports chi.Router) {
				reports.Get("/summary", reportHandler.Summary)
				reports.Get("/cash-flow", reportHandler.CashFlow)
				reports.Get("/cash-flow/excel", reportHandler.CashFlowExcel)
				reports.Get("/cash-flow/pdf", reportHandler.CashFlowPDF)
				reports.Get("/expense-by-category", reportHandler.ExpenseByCategory)
				reports.Get("/monthly", reportHandler.Monthly)
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		utils.Error(w, http.StatusNotFound, "Endpoint not found", nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		utils.Error(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
	})

	return r
}
