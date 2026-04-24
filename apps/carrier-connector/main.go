package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/config"
	"github.com/nutcas3/telecom-platform/apps/carrier-connector/internal/es2"
	"github.com/rs/zerolog"
)

var logger zerolog.Logger

// GSMA ES2+ Profile Order Request
type ProfileOrder struct {
	EID              string `json:"eid"`
	ICCID            string `json:"iccid"`
	IMSI             string `json:"imsi"`
	K                string `json:"k"`
	OPc              string `json:"opc"`
	MCC              string `json:"mcc"`
	MNC              string `json:"mnc"`
	ProfileType      string `json:"profileType"`
	ConfirmationCode string `json:"confirmationCode,omitempty"`
}

// GSMA ES2+ Profile Order Response
type ProfileResponse struct {
	ExecutionStatus string `json:"executionStatus"`
	StatusMessage   string `json:"statusMessage"`
	ProfileID       string `json:"profileId"`
	ActivationCode  string `json:"activationCode,omitempty"`
}

func main() {
	// Initialize logger
	logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "carrier-connector").
		Logger()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Info().Msg("No .env file found, using system environment")
	}

	// Initialize ES2+ client with GSMA protocol
	smdpURL := getEnv("SMDP_URL", "https://smdp.example.com")
	apiKey := getEnv("SMDP_API_KEY", "test-api-key")
	requesterID := getEnv("FUNCTIONALITY_REQUESTER_ID", "carrier-connector")
	insecure := getEnv("INSECURE_SKIP_VERIFY", "false") == "true"
	port := getEnv("PORT", "8080")

	es2Config := &config.ES2Config{
		BaseURL:                  smdpURL,
		APIKey:                   apiKey,
		FunctionalityRequesterID: requesterID,
		InsecureSkipVerify:       insecure,
	}

	client := es2.NewES2Client(es2Config)

	// Initialize Gin router
	router := gin.Default()

	// Add middleware for logging
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Initialize API routes
	setupRoutes(router, client)

	logger.Info().
		Str("port", port).
		Msg("Carrier Connector API server starting")

	// Start server
	if err := router.Run(":" + port); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server")
	}
}

func setupRoutes(router *gin.Engine, client *ES2Client) {
	api := router.Group("/api/v1")

	// Health check endpoint
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "carrier-connector",
			"timestamp": time.Now().UTC(),
		})
	})

	// eSIM profile endpoints
	esim := api.Group("/esim")
	{
		// Order a new eSIM profile
		esim.POST("/profiles", orderProfileHandler(client))

		// Get profile status
		esim.GET("/profiles/:profileId", getProfileHandler(client))

		// List all profiles
		esim.GET("/profiles", listProfilesHandler(client))

		// Delete/disable profile
		esim.DELETE("/profiles/:profileId", deleteProfileHandler(client))
	}

	// Carrier management endpoints
	carrier := api.Group("/carrier")
	{
		// Get carrier info
		carrier.GET("/info", getCarrierInfoHandler(client))

		// Check SM-DP+ connectivity
		carrier.GET("/connectivity", checkConnectivityHandler(client))
	}
}

// API Handlers

func orderProfileHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var order ProfileOrder

		if err := c.ShouldBindJSON(&order); err != nil {
			logger.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"message": err.Error(),
			})
			return
		}

		// Validate required fields
		if order.ICCID == "" || order.IMSI == "" || order.K == "" || order.OPc == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing required fields",
				"message": "ICCID, IMSI, K, and OPc are required",
			})
			return
		}

		logger.Info().
			Str("imsi", order.IMSI).
			Str("iccid", order.ICCID).
			Msg("API request: Ordering eSIM profile from SM-DP+")

		// Convert ProfileOrder to DownloadProfileRequest for GSMA ES2+ protocol
		downloadReq := &es2.DownloadProfileRequest{
			EID:              order.EID,
			ICCID:            order.ICCID,
			ProfileType:      order.ProfileType,
			ConfirmationCode: order.ConfirmationCode,
		}

		downloadResp, err := client.DownloadProfile(context.Background(), downloadReq)
		if err != nil {
			logger.Error().Err(err).Str("imsi", order.IMSI).Msg("Failed to order profile")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to order profile",
				"message": err.Error(),
			})
			return
		}

		logger.Info().
			Str("execution_status", downloadResp.ExecutionStatus).
			Str("status_message", downloadResp.StatusMessage).
			Str("imsi", order.IMSI).
			Msg("Profile ordered successfully via API")

		// Convert GSMA response to our API response format
		response := &ProfileResponse{
			ExecutionStatus: downloadResp.ExecutionStatus,
			StatusMessage:   downloadResp.StatusMessage,
			ProfileID:       order.ICCID, // Use ICCID as profile ID for our API
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"profile": response,
		})
	}
}

func getProfileHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		profileID := c.Param("profileId")

		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing profile ID",
				"message": "Profile ID is required",
			})
			return
		}

		logger.Info().
			Str("profile_id", profileID).
			Msg("API request: Getting eSIM profile status")

		// TODO: Implement actual profile retrieval from SM-DP+
		// For now, return a mock response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"profile": gin.H{
				"profileId": profileID,
				"status":    "active",
				"createdAt": time.Now().UTC(),
			},
		})
	}
}

func listProfilesHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info().Msg("API request: Listing eSIM profiles")

		// Parse query parameters
		page := 1
		if pageStr := c.Query("page"); pageStr != "" {
			if parsed, err := strconv.Atoi(pageStr); err == nil && parsed > 0 {
				page = parsed
			}
		}

		limit := 20
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		// TODO: Implement actual profile listing from database/SM-DP+
		// For now, return a mock response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"profiles": []gin.H{
				{
					"profileId": "profile-123",
					"imsi":      "208930000000001",
					"status":    "active",
				},
				{
					"profileId": "profile-456",
					"imsi":      "208930000000002",
					"status":    "inactive",
				},
			},
			"pagination": gin.H{
				"page":  page,
				"limit": limit,
				"total": 2,
			},
		})
	}
}

func deleteProfileHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		profileID := c.Param("profileId")

		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Missing profile ID",
				"message": "Profile ID is required",
			})
			return
		}

		logger.Info().
			Str("profile_id", profileID).
			Msg("API request: Deleting eSIM profile")

		// TODO: Implement actual profile deletion/disable from SM-DP+
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Profile deleted successfully",
		})
	}
}

func getCarrierInfoHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info().Msg("API request: Getting carrier information")

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"carrier": gin.H{
				"name":        "Example Carrier",
				"mcc":         "208",
				"mnc":         "93",
				"smdpUrl":     "https://smdp.example.com", // Use config value
				"supported":   true,
				"lastChecked": time.Now().UTC(),
			},
		})
	}
}

func checkConnectivityHandler(client *es2.ES2Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info().Msg("API request: Checking SM-DP+ connectivity")

		// Simple connectivity check using the ES2+ client
		// Use a GetProfileStatus request with dummy data to test connectivity
		req := &es2.GetProfileStatusRequest{
			EID:   "test-eid",
			ICCID: "test-iccid",
		}

		_, err := client.GetProfileStatus(context.Background(), req)

		isConnected := err == nil
		statusCode := 200
		if err != nil {
			statusCode = 500 // Assume connection error
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"connectivity": gin.H{
				"smdpConnected": isConnected,
				"statusCode":    statusCode,
				"checkedAt":     time.Now().UTC(),
				"error":         nil,
			},
		})
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
