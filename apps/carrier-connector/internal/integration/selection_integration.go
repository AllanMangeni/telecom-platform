package integration

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/config"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/handlers"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/repository"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/service"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/smdp"
)

// SelectionIntegration wires together the carrier selection components
type SelectionIntegration struct {
	manager          *smdp.SMDPManager
	selectionService *service.SelectionService
	selectionHandler *handlers.SelectionHandler
	smdpService      *service.SMDPService
	server           *http.Server
}

// NewSelectionIntegration creates a new selection integration
func NewSelectionIntegration(repo *repository.PostgresProfileStore) *SelectionIntegration {
	// Create SMDP manager with default configuration
	config := &smdp.ManagerConfig{
		HealthCheckInterval: 30 * time.Second,
		EnableFailover:      true,
		EnableLoadBalancing: true,
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
	}

	manager := smdp.NewSMDPManager(repo, config)

	// Create selection service
	selectionService := service.NewSelectionService(manager)

	// Create SMDP service
	smdpService := service.NewSMDPService(repo)

	return &SelectionIntegration{
		manager:          manager,
		selectionService: selectionService,
		selectionHandler: selectionService.GetHandler(),
		smdpService:      smdpService,
	}
}

// SetupCarriers configures default carriers for demonstration
func (si *SelectionIntegration) SetupCarriers() error {
	// Configure sample carriers with different characteristics
	carriers := []*smdp.Carrier{
		{
			ID:          "att-us",
			Name:        "AT&T US",
			MCC:         "310",
			MNC:         "410",
			CountryCode: "US",
			IsActive:    true,
			Priority:    90,
			ES2Config: &config.ES2Config{
				BaseURL:                  "https://es2plus.att.com",
				APIKey:                   "demo-key-att",
				InsecureSkipVerify:       false,
				FunctionalityRequesterID: "telecom-platform",
			},
			Capabilities: &smdp.CarrierCapabilities{
				SupportedProfileTypes: []string{"operational", "testing"},
				Features:              []string{"bulk_download", "remote_provisioning"},
				MaxConcurrentRequests: 100,
			},
			Metrics: &smdp.CarrierMetrics{
				TotalRequests:       1000,
				SuccessfulRequests:  980,
				FailedRequests:      20,
				AverageResponseTime: 150 * time.Millisecond,
				RequestRate:         10.5,
			},
		},
		{
			ID:          "verizon-us",
			Name:        "Verizon US",
			MCC:         "311",
			MNC:         "480",
			CountryCode: "US",
			IsActive:    true,
			Priority:    85,
			ES2Config: &config.ES2Config{
				BaseURL:                  "https://es2plus.verizon.com",
				APIKey:                   "demo-key-verizon",
				InsecureSkipVerify:       false,
				FunctionalityRequesterID: "telecom-platform",
			},
			Capabilities: &smdp.CarrierCapabilities{
				SupportedProfileTypes: []string{"operational", "testing"},
				Features:              []string{"bulk_download"},
				MaxConcurrentRequests: 80,
			},
			Metrics: &smdp.CarrierMetrics{
				TotalRequests:       800,
				SuccessfulRequests:  790,
				FailedRequests:      10,
				AverageResponseTime: 120 * time.Millisecond,
				RequestRate:         8.2,
			},
		},
		{
			ID:          "tmobile-de",
			Name:        "T-Mobile Germany",
			MCC:         "262",
			MNC:         "01",
			CountryCode: "DE",
			IsActive:    true,
			Priority:    75,
			ES2Config: &config.ES2Config{
				BaseURL:                  "https://es2plus.t-mobile.de",
				APIKey:                   "demo-key-tmobile",
				InsecureSkipVerify:       false,
				FunctionalityRequesterID: "telecom-platform",
			},
			Capabilities: &smdp.CarrierCapabilities{
				SupportedProfileTypes: []string{"operational"},
				Features:              []string{"remote_provisioning"},
				MaxConcurrentRequests: 60,
			},
			Metrics: &smdp.CarrierMetrics{
				TotalRequests:       600,
				SuccessfulRequests:  570,
				FailedRequests:      30,
				AverageResponseTime: 200 * time.Millisecond,
				RequestRate:         6.8,
			},
		},
		{
			ID:          "orange-fr",
			Name:        "Orange France",
			MCC:         "208",
			MNC:         "01",
			CountryCode: "FR",
			IsActive:    true,
			Priority:    70,
			ES2Config: &config.ES2Config{
				BaseURL:                  "https://es2plus.orange.fr",
				APIKey:                   "demo-key-orange",
				InsecureSkipVerify:       false,
				FunctionalityRequesterID: "telecom-platform",
			},
			Capabilities: &smdp.CarrierCapabilities{
				SupportedProfileTypes: []string{"operational", "testing"},
				Features:              []string{},
				MaxConcurrentRequests: 50,
			},
			Metrics: &smdp.CarrierMetrics{
				TotalRequests:       400,
				SuccessfulRequests:  380,
				FailedRequests:      20,
				AverageResponseTime: 180 * time.Millisecond,
				RequestRate:         4.5,
			},
		},
	}

	// Add carriers to the manager
	for _, carrier := range carriers {
		if err := si.manager.AddCarrier(carrier); err != nil {
			return err
		}
	}

	log.Printf("Added %d carriers to the selection manager", len(carriers))
	return nil
}

// StartServer starts the HTTP server with all endpoints
func (si *SelectionIntegration) StartServer(port string) error {
	// Create HTTP multiplexer
	mux := http.NewServeMux()

	// Register selection routes
	si.selectionHandler.RegisterRoutes(mux)

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "selection-integration"}`))
	})

	// Create and configure server
	si.server = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting selection integration server on port %s", port)
	return si.server.ListenAndServe()
}

// StartHealthChecking starts the background health checking process
func (si *SelectionIntegration) StartHealthChecking(ctx context.Context) {
	si.manager.StartHealthChecking(ctx)
}

// GetManager returns the SMDP manager for testing
func (si *SelectionIntegration) GetManager() *smdp.SMDPManager {
	return si.manager
}

// GetSelectionService returns the selection service for testing
func (si *SelectionIntegration) GetSelectionService() *service.SelectionService {
	return si.selectionService
}

// Shutdown gracefully shuts down the server
func (si *SelectionIntegration) Shutdown(ctx context.Context) error {
	if si.server != nil {
		return si.server.Shutdown(ctx)
	}
	return nil
}

// RunDemo runs a demonstration of the carrier selection capabilities
func (si *SelectionIntegration) RunDemo(ctx context.Context) error {
	log.Println("Starting carrier selection demonstration...")

	// Setup carriers
	if err := si.SetupCarriers(); err != nil {
		return err
	}

	// Start health checking
	si.StartHealthChecking(ctx)

	// Wait a moment for health checks to initialize
	time.Sleep(2 * time.Second)

	// Demonstrate intelligent carrier selection
	log.Println("Demonstrating intelligent carrier selection...")

	// Test different selection scenarios
	scenarios := []struct {
		name     string
		criteria *smdp.SelectionCriteria
	}{
		{
			name: "High Priority US Request",
			criteria: &smdp.SelectionCriteria{
				Region:            "US",
				ProfileType:       "operational",
				Urgency:           "high",
				CostSensitivity:   0.2,
				PerformanceWeight: 0.6,
				ReliabilityWeight: 0.6,
			},
		},
		{
			name: "Cost-Optimized European Request",
			criteria: &smdp.SelectionCriteria{
				Region:            "DE",
				ProfileType:       "operational",
				Urgency:           "low",
				CostSensitivity:   0.8,
				PerformanceWeight: 0.2,
				ReliabilityWeight: 0.3,
			},
		},
		{
			name: "Balanced Global Request",
			criteria: &smdp.SelectionCriteria{
				Region:            "",
				ProfileType:       "operational",
				Urgency:           "medium",
				CostSensitivity:   0.5,
				PerformanceWeight: 0.4,
				ReliabilityWeight: 0.4,
			},
		},
	}

	for _, scenario := range scenarios {
		log.Printf("Testing scenario: %s", scenario.name)

		score, err := si.manager.SelectOptimalCarrier(ctx, scenario.criteria)
		if err != nil {
			log.Printf("Error in scenario %s: %v", scenario.name, err)
			continue
		}

		log.Printf("Selected carrier: %s (%s)", score.Carrier.Name, score.CarrierID)
		log.Printf("Total score: %.2f", score.TotalScore)
		log.Printf("Performance: %.2f, Reliability: %.2f, Cost: %.2f, Region: %.2f, Capability: %.2f",
			score.PerformanceScore, score.ReliabilityScore, score.CostScore, score.RegionScore, score.CapabilityScore)
		log.Printf("Reason: %s", score.Reason)
		log.Println("---")
	}

	// Demonstrate analytics
	log.Println("Generating carrier analytics...")
	analytics, err := si.selectionService.GetCarrierAnalytics(ctx)
	if err != nil {
		log.Printf("Error generating analytics: %v", err)
	} else {
		log.Printf("Analytics generated for %d carriers", analytics.TotalCarriers)
		log.Printf("Overall health: %.1f%%", analytics.Summary.OverallHealth)
		log.Printf("Overall success rate: %.1f%%", analytics.Summary.OverallSuccessRate)
	}

	// Demonstrate optimization recommendations
	log.Println("Generating optimization recommendations...")
	optimization, err := si.selectionService.OptimizeCarrierSelection(ctx)
	if err != nil {
		log.Printf("Error generating optimization: %v", err)
	} else {
		log.Printf("System health: %.1f%%", optimization.OverallHealth)
		log.Printf("Recommendations: %d", len(optimization.Recommendations))
		log.Printf("Priority actions: %d", len(optimization.PriorityActions))
	}

	log.Println("Carrier selection demonstration completed successfully!")
	return nil
}
