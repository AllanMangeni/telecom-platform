package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/es2"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/handlers"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/mq"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/repository"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/services"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/smdp"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/webhook"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// setupRoutes registers all HTTP routes, wiring the ES2+ client, profile repo, webhook client, and message queue.
func setupRoutes(router *gin.Engine, client *es2.ES2Client, profileRepo repository.ProfileRepository, webhookClient *webhook.WebhookClient, messageQueue *mq.MessageQueue, repo repository.Repository, db *gorm.DB, logger *logrus.Logger) {
	api := router.Group("/api/v1")
	api.GET("/health", healthHandler)
	api.GET("/health/ready", readinessHandler(profileRepo))
	api.GET("/health/live", livenessHandler)
	api.GET("/metrics", gin.WrapH(promhttp.Handler()))

	esim := api.Group("/esim")
	{
		esim.POST("/profiles", handlers.OrderProfileHandlerWithRepo(client, profileRepo, webhookClient, messageQueue))
		esim.GET("/profiles", handlers.ListProfilesHandler(profileRepo))
		esim.GET("/profiles/:profileId", handlers.GetProfileHandler(profileRepo))
		esim.DELETE("/profiles/:profileId", handlers.DeleteProfileHandler(profileRepo))
	}

	carrier := api.Group("/carrier")
	{
		carrier.GET("/info", handlers.GetCarrierInfoHandler(client))
		carrier.GET("/connectivity", handlers.CheckConnectivityHandler(client))
	}

	// MVNO routes
	mvno := api.Group("/mvno")
	{
		// Create MVNO handlers
		onboardingService := services.NewOnboardingService(logger)
		mvnoHandler := handlers.NewMVNOHandler(onboardingService, repo, logger)
		managementHandler := handlers.NewManagementHandler(repo, logger)

		// MVNO onboarding and management routes
		mvno.POST("/onboarding", mvnoHandler.StartOnboarding)
		mvno.GET("", mvnoHandler.ListMVNOs)
		mvno.GET("/:id", mvnoHandler.GetMVNO)
		mvno.PUT("/:id/status", managementHandler.UpdateMVNOStatus)
		mvno.GET("/stats", managementHandler.GetMVNOStats)
	}

	// Rate Plan routes
	rateplanGroup := api.Group("/rateplans")
	{
		// Initialize rate plan service and handler
		ratePlanService := services.NewService(repo, logger)
		ratePlanAdapter := services.NewRatePlanAdapter(ratePlanService)
		ratePlanHandler := handlers.NewRatePlanHandler(ratePlanAdapter)

		// Rate plan CRUD operations
		rateplanGroup.POST("", ratePlanHandler.CreateRatePlan)
		rateplanGroup.GET("", ratePlanHandler.ListRatePlans)
		rateplanGroup.GET("/:id", ratePlanHandler.GetRatePlan)
		rateplanGroup.PUT("/:id", ratePlanHandler.UpdateRatePlan)
		rateplanGroup.DELETE("/:id", ratePlanHandler.DeleteRatePlan)

		// Rate plan search
		rateplanGroup.GET("/search", ratePlanHandler.SearchRatePlans)

		// Rate plan subscription operations
		rateplanGroup.POST("/subscribe", ratePlanHandler.SubscribeToPlan)
		rateplanGroup.GET("/subscriptions", ratePlanHandler.ListSubscriptions)
		rateplanGroup.GET("/subscriptions/:id", ratePlanHandler.GetSubscription)
		rateplanGroup.GET("/subscriptions/active", ratePlanHandler.GetActiveSubscription)
		rateplanGroup.DELETE("/subscriptions/:id", ratePlanHandler.CancelSubscription)

		// Rate plan management
		rateplanGroup.GET("/dashboard", ratePlanHandler.GetManagementDashboard)
		rateplanGroup.GET("/overview", ratePlanHandler.GetSystemOverview)
		rateplanGroup.POST("/bulk", ratePlanHandler.BulkCreateRatePlans)
		rateplanGroup.PUT("/:id/activate", ratePlanHandler.ActivateRatePlan)
		rateplanGroup.PUT("/:id/deactivate", ratePlanHandler.DeactivateRatePlan)
		rateplanGroup.POST("/:id/duplicate", ratePlanHandler.DuplicateRatePlan)
	}

	// Pricing routes
	pricingGroup := api.Group("/pricing")
	{
		// Initialize pricing service and handler
		// TODO: Create pricing repository, engine, and validator
		// For now, we'll use a simple implementation
		pricingService := services.NewPricingService(nil, nil, nil, logger)
		pricingHandler := handlers.NewPricingHandler(pricingService)

		// Pricing rules management
		pricingGroup.POST("/rules", pricingHandler.CreateRule)
		pricingGroup.GET("/rules", pricingHandler.ListRules)
		pricingGroup.GET("/rules/:id", pricingHandler.GetRule)
		pricingGroup.PUT("/rules/:id", pricingHandler.UpdateRule)
		pricingGroup.DELETE("/rules/:id", pricingHandler.DeleteRule)
	}

	// Currency and Billing routes
	currencyGroup := api.Group("/currency")
	{
		// TODO: Initialize currency services and handler when available
		// For now, using placeholder implementations

		// Currency conversion and exchange rates
		currencyGroup.POST("/convert", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Currency conversion endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/exchange/:from/:to", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Exchange rate endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/exchange/:from/:to/history", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Exchange rate history endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/currencies", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Supported currencies endpoint ready", "status": "placeholder"})
		})
		currencyGroup.POST("/exchange/refresh", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Exchange rate refresh endpoint ready", "status": "placeholder"})
		})

		// Billing operations
		currencyGroup.POST("/billing", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Billing processing endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/billing/history/:profile_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Billing history endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/billing/summary/:profile_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Billing summary endpoint ready", "status": "placeholder"})
		})
		currencyGroup.POST("/billing/refund/:transaction_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Billing refund endpoint ready", "status": "placeholder"})
		})
		currencyGroup.GET("/billing/analytics", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Billing analytics endpoint ready", "status": "placeholder"})
		})
	}

	// Tenant routes
	tenant := api.Group("/tenants")
	{
		// Initialize tenant service and handler
		tenantRepo := repository.NewGormTenantRepository(db)
		// TODO: Implement proper rate limiter - using nil for now
		tenantService := services.NewTenantService(tenantRepo, nil, logger)
		tenantHandler := handlers.NewTenantHandler(tenantService, logger)

		// Tenant management
		tenant.POST("", tenantHandler.CreateTenant)
		tenant.GET("", tenantHandler.ListTenants)
		tenant.GET("/:id", tenantHandler.GetTenant)
		tenant.GET("/domain/:domain", tenantHandler.GetTenantByDomain)
		tenant.PUT("/:id", tenantHandler.UpdateTenant)
		tenant.DELETE("/:id", tenantHandler.DeleteTenant)

		// Tenant user management
		tenant.POST("/:id/users", tenantHandler.AddUserToTenant)
		tenant.GET("/:id/users", tenantHandler.ListTenantUsers)
		tenant.GET("/:id/users/:user_id", tenantHandler.GetTenantUser)
		tenant.PUT("/:id/users/:user_id", tenantHandler.UpdateTenantUser)
		tenant.DELETE("/:id/users/:user_id", tenantHandler.RemoveUserFromTenant)

		// Tenant API key management
		tenant.POST("/:id/apikeys", tenantHandler.CreateAPIKey)
		tenant.GET("/:id/apikeys", tenantHandler.ListAPIKeys)
		tenant.GET("/:id/apikeys/:key_id", tenantHandler.GetAPIKey)
		tenant.PUT("/:id/apikeys/:key_id", tenantHandler.UpdateAPIKey)
		tenant.DELETE("/:id/apikeys/:key_id", tenantHandler.DeleteAPIKey)

		// Tenant metrics and configuration
		tenant.GET("/:id/usage", tenantHandler.GetUsageStats)
		tenant.GET("/:id/quota", tenantHandler.GetQuotaStatus)
		tenant.GET("/:id/config", tenantHandler.GetTenantConfig)
		tenant.PUT("/:id/config", tenantHandler.UpdateTenantConfig)
		tenant.GET("/:id/metrics", tenantHandler.GetTenantMetrics)
		tenant.GET("/:id/events", tenantHandler.GetTenantEvents)
	}

	// Initialize SMDP manager for both SMDP and selection routes
	smdpConfig := smdp.DefaultManagerConfig()
	smdpManager := smdp.NewSMDPManager(profileRepo.(*repository.PostgresProfileStore), smdpConfig)

	// SMDP management routes
	smdpGroup := api.Group("/smdp")
	{
		// Initialize SMDP handler
		smdpHandler := handlers.NewSMDPHandler(smdpManager)

		// SMDP operations
		smdpGroup.DELETE("/carriers/:carrier_id", smdpHandler.RemoveCarrier)
		smdpGroup.GET("/carriers/:carrier_id/history", smdpHandler.GetSelectionHistory)
		smdpGroup.PUT("/carriers/:carrier_id/learning", smdpHandler.UpdateLearning)
	}

	// Selection and Analytics routes
	selection := api.Group("/selection")
	{
		// Initialize selection handler
		selectionHandler := handlers.NewSelectionHandler(smdpManager)

		// Carrier selection - wrap handlers for gin compatibility
		selection.POST("/optimal", func(c *gin.Context) {
			selectionHandler.SelectOptimalCarrier(c.Writer, c.Request)
		})
		selection.GET("/default", func(c *gin.Context) {
			selectionHandler.SelectCarrier(c.Writer, c.Request)
		})
		selection.GET("/analytics", func(c *gin.Context) {
			selectionHandler.GetSelectionAnalytics(c.Writer, c.Request)
		})
	}
}

// healthHandler returns a simple liveness response.
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "carrier-connector",
		"timestamp": time.Now().UTC(),
	})
}

// livenessHandler returns a simple liveness check (always healthy if service is running).
func livenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"service":   "carrier-connector",
		"timestamp": time.Now().UTC(),
	})
}

// readinessHandler checks if the service is ready to accept requests (database connectivity).
func readinessHandler(repo repository.ProfileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check database connectivity
		if err := repo.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":    "not ready",
				"service":   "carrier-connector",
				"timestamp": time.Now().UTC(),
				"error":     "database connection failed",
				"details":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"service":   "carrier-connector",
			"timestamp": time.Now().UTC(),
			"checks": gin.H{
				"database": "ok",
			},
		})
	}
}
