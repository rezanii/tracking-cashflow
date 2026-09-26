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
	// webhookRateLimit is higher than the credential limit because Telegram can legitimately
	// deliver a burst of updates, but it still bounds an unauthenticated endpoint.
	webhookRateLimit = 120
)

// webhookSecret is only handed to the handler in webhook mode. Anywhere else the endpoint has
// no secret to compare against and answers 404, so a stale secret in the environment cannot
// leave a publicly reachable endpoint accepting updates the service would not act on.
func webhookSecret(cfg config.Config) string {
	if cfg.Telegram.Mode != config.TelegramModeWebhook {
		return ""
	}
	return cfg.Telegram.WebhookSecret
}

// NewTelegramService builds the bot service from configuration. It is exported from this
// package so cmd/api can start the poller with the same instance the router serves.
func NewTelegramService(
	cfg config.Config,
	links repository.TelegramRepository,
	reports service.DailyReportService,
	accounts repository.AccountRepository,
) service.TelegramService {
	client := service.NewTelegramClient(cfg.Telegram.BotToken, cfg.Telegram.APIBaseURL)
	return service.NewTelegramService(
		client,
		links,
		reports,
		accounts,
		cfg.Telegram.PairingCodeTTL,
		cfg.Telegram.WebhookURL,
		cfg.Telegram.WebhookSecret,
		cfg.Telegram.Mode == config.TelegramModeWebhook,
	)
}

// New wires the dependency graph and returns the HTTP handler. Construction happens once at
// startup, so every request reuses the same services and connection pool.
func New(cfg config.Config, db *gorm.DB) http.Handler {
	tokens := utils.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiration)
	requestValidator := validator.New()

	users := repository.NewUserRepository(db)
	categories := repository.NewCategoryRepository(db)
	transactions := repository.NewTransactionRepository(db)
	accounts := repository.NewAccountRepository(db)
	reports := repository.NewReportRepository(db)
	dailyReports := repository.NewDailyReportRepository(db)
	telegramLinks := repository.NewTelegramRepository(db)
	txManager := repository.NewTxManager(db)

	authService := service.NewAuthService(users, tokens)
	categoryService := service.NewCategoryService(categories)
	accountService := service.NewAccountService(accounts)
	transactionService := service.NewTransactionService(transactions, categories, accounts, txManager)
	reportService := service.NewReportService(reports, transactions)
	dailyReportService := service.NewDailyReportService(dailyReports, accounts)
	telegramService := NewTelegramService(cfg, telegramLinks, dailyReportService, accounts)

	authHandler := handler.NewAuthHandler(authService, requestValidator)
	categoryHandler := handler.NewCategoryHandler(categoryService, requestValidator)
	accountHandler := handler.NewAccountHandler(accountService, requestValidator)
	transactionHandler := handler.NewTransactionHandler(transactionService, requestValidator)
	reportHandler := handler.NewReportHandler(reportService, dailyReportService)
	telegramHandler := handler.NewTelegramHandler(telegramService, webhookSecret(cfg))

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

		// Telegram calls this one, so it cannot carry a JWT. It authenticates with the
		// secret header instead and is rate limited because it is publicly reachable.
		api.Group(func(public chi.Router) {
			public.Use(httprate.LimitByIP(webhookRateLimit, authRateWindow))
			public.Post("/telegram/webhook", telegramHandler.Webhook)
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

			private.Route("/accounts", func(accounts chi.Router) {
				accounts.Get("/", accountHandler.List)
				accounts.Post("/", accountHandler.Create)
				accounts.Get("/{id}", accountHandler.Get)
				accounts.Put("/{id}", accountHandler.Update)
				accounts.Patch("/{id}/status", accountHandler.SetStatus)
				accounts.Delete("/{id}", accountHandler.Delete)
				accounts.Get("/{id}/balances", accountHandler.ListBalances)
				accounts.Post("/{id}/balances", accountHandler.RecordBalance)
				accounts.Delete("/{id}/balances/{balance_id}", accountHandler.DeleteBalance)
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
				reports.Get("/daily-cash-flow", reportHandler.DailyCashFlow)
			})

			private.Route("/telegram", func(telegram chi.Router) {
				telegram.Post("/pairing-code", telegramHandler.PairingCode)
				telegram.Get("/link", telegramHandler.Link)
				telegram.Delete("/link", telegramHandler.Unlink)
				telegram.Post("/send/daily-report", telegramHandler.SendDailyReport)
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
