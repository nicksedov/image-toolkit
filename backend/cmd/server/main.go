package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"image-toolkit/internal/application/auth"
	"image-toolkit/internal/application/imaging"
	"image-toolkit/internal/application/thumbnail"
	agentpkg "image-toolkit/internal/application/agent"
	"image-toolkit/internal/domain"
	"image-toolkit/internal/infrastructure/config"
	"image-toolkit/internal/infrastructure/database"
	"image-toolkit/internal/infrastructure/geocoder"
	"image-toolkit/internal/infrastructure/mcpserver"
	"image-toolkit/internal/infrastructure/ocr"
	"image-toolkit/internal/interfaces/handler"
	"image-toolkit/internal/interfaces/handler/helpers"
	"image-toolkit/internal/interfaces/i18n"
	"image-toolkit/internal/interfaces/middleware"
)

// init is invoked before main()
func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	fmt.Printf("Image Dedup - API Server\n")
	fmt.Printf("========================\n\n")

	// Initialize database
	fmt.Println("Connecting to PostgreSQL database...")
	db, err := database.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	fmt.Println("Database connected successfully!")

	// Check for exiftool availability
	fmt.Println("Checking for exiftool...")
	if _, err := exec.LookPath("exiftool"); err != nil {
		log.Fatalf("exiftool not found in PATH. Please install exiftool: https://exiftool.org/")
	}
	fmt.Println("exiftool found!")

	// Initialize exiftool for EXIF extraction
	if err := imaging.InitExifTool(); err != nil {
		log.Fatalf("Failed to initialize exiftool: %v", err)
	}
	fmt.Println("exiftool initialized!")

	// Initialize OCR classifier client and health check
	var ocrCheckInterval int
	if cfg.OCREnabled {
		fmt.Println("Initializing OCR classifier client...")
		ocrCheckInterval = cfg.OCRCheckInterval
		fmt.Printf("OCR classifier enabled: host=%s, port=%s, check interval=%ds\n", cfg.OCRHost, cfg.OCRPort, ocrCheckInterval)
	} else {
		fmt.Println("OCR classifier integration disabled")
		ocrCheckInterval = 0
	}

	// Create scan manager (reads gallery folders from DB dynamically)
	scanManager := imaging.NewScanManager(db, cfg.ScanWorkers)

	// Create OCR manager (background classification)
	var ocrManager *imaging.OcrManager
	if cfg.OCREnabled {
		ocrClient := ocr.NewClient(cfg.OCRHost, cfg.OCRPort)

		// Read OCR concurrent requests from DB, fallback to env var (default: 4)
		ocrWorkers := cfg.OCRConcurrentRequests
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.OcrConcurrentRequests > 0 {
			ocrWorkers = appSettings.OcrConcurrentRequests
		}

		ocrManager = imaging.NewOcrManager(db, ocrClient, ocrWorkers)
		fmt.Printf("OCR manager initialized: max concurrent requests=%d\n", ocrWorkers)
	}

	// Initialize thumbnail cache service
	var thumbnailService *thumbnail.Service
	// Initialize thumbnail cache service
	fmt.Println("Initializing thumbnail cache service...")

	// Load thumbnail cache path from database if available
	cachePath := cfg.ThumbnailCachePath
	if cfg.ThumbnailCachePath == "" {
		// Check database for saved cache path
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.ThumbnailCachePath != "" {
			cachePath = appSettings.ThumbnailCachePath
			fmt.Printf("Using thumbnail cache path from database: %s\n", cachePath)
		}
	}

	tcConfig := &thumbnail.Config{
		CacheDir:      cachePath,
		MaxSize:       cfg.ThumbnailCacheMaxSize,
		Quality:       cfg.ThumbnailCacheQuality,
		Enabled:       cfg.ThumbnailCacheEnabled,
		Format:        "webp",
		PreloadOnScan: cfg.ThumbnailCachePreloadOnScan,
	}
	thumbnailService, err = thumbnail.NewService(tcConfig)
	if err != nil {
		log.Printf("Failed to initialize thumbnail cache: %v", err)
		thumbnailService = nil
	} else {
		if cfg.ThumbnailCacheEnabled {
			fmt.Println("Thumbnail cache service initialized and enabled")
		} else {
			fmt.Println("Thumbnail cache service initialized (disabled)")
		}
	}

	// Start thumbnail service
	if thumbnailService != nil {
		if err := thumbnailService.Start(); err != nil {
			log.Printf("Failed to start thumbnail service: %v", err)
		}
	}

	// Initialize Nominatim client for geocoding (forward search + reverse geocoding)
	nominatimClient := geocoder.NewNominatimClient(nil, "")
	fmt.Println("Nominatim geocoding client initialized")

	// Initialize GeolocationService (cache + rate-limited Nominatim reverse geocoding)
	geolocationService := geocoder.NewGeolocationService(db, nominatimClient)
	fmt.Println("Geolocation service initialized (Nominatim-backed cache)")

	// Create background sync manager
	backgroundSync := imaging.NewBackgroundSyncManager(db, thumbnailService, geolocationService)

	// Read schedule from DB (default: enabled=true, hour=3, minute=30)
	syncEnabled := cfg.BackgroundSyncEnabled
	syncHour := 3
	syncMinute := 30
	var appSettings domain.AppSettings
	if result := db.First(&appSettings, 1); result.Error == nil {
		if appSettings.DailySyncHour > 0 || appSettings.DailySyncMinute > 0 {
			syncHour = appSettings.DailySyncHour
			syncMinute = appSettings.DailySyncMinute
		}
		// Use DB value for enabled flag if it has been set (default true)
		syncEnabled = appSettings.DailySyncEnabled
	}

	backgroundSync.Start(syncEnabled, syncHour, syncMinute)
	defer backgroundSync.Stop()
	fmt.Printf("Background sync: daily at %02d:%02d, enabled=%v\n", syncHour, syncMinute, syncEnabled)

	// Wire scan complete callback to trigger OCR classification
	scanManager.OnScanComplete = func() {
		if cfg.OCREnabled && ocrManager != nil {
			if err := ocrManager.StartClassification(false); err != nil {
				log.Printf("OCR classification not started: %v", err)
			}
		}
	}

	// Initialize i18n service
	i18nSvc, err := i18n.NewService()
	if err != nil {
		log.Fatalf("Failed to initialize i18n service: %v", err)
	}
	fmt.Println("i18n service initialized with English and Russian translations")

	// Initialize authentication components
	sessionConfig := &auth.SessionConfig{
		IdleTimeout:     time.Duration(cfg.SessionIdleHours) * time.Hour,
		AbsoluteTimeout: time.Duration(cfg.SessionAbsoluteDays) * 24 * time.Hour,
		CookieMaxAge:    cfg.SessionIdleHours * 60 * 60,
		TokenLength:     64,
	}

	sessionRepo := auth.NewSessionRepository(db, sessionConfig)
	bootstrap := auth.NewBootstrapService(db, cfg.BootstrapLogin, cfg.BootstrapPassword)
	loginLimiter := auth.NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	authService := auth.NewAuthService(db, bootstrap, sessionRepo, loginLimiter)
	userService := auth.NewUserService(db, sessionRepo)
	authMiddleware := middleware.NewAuthMiddleware(sessionRepo, authService, i18nSvc)
	csrfProtection := middleware.NewCSRFProtection(i18nSvc)
	authHandlers := handler.NewAuthHandlers(authService, bootstrap, userService, sessionRepo, db, i18nSvc)
	// Start session cleanup job
	sessionCleanup := auth.NewSessionCleanupJob(sessionRepo, 1*time.Hour)
	sessionCleanup.Start()
	defer sessionCleanup.Stop()

	fmt.Println("Authentication system initialized!")

	// Create LLM OCR service
	llmOcrService := imaging.NewLlmOcrService(db)
	fmt.Println("LLM OCR service initialized")

	// Create tag scan manager
	tagScanManager := imaging.NewTagScanManager(db, llmOcrService, cfg.LlmMaxImageMegapixels)

	// Read tag scan schedule from LlmSettings
	tagScanEnabled := true
	tagScanStartHour := 22
	tagScanStartMinute := 0
	tagScanEndHour := 7
	tagScanEndMinute := 0
	tagScanTimezoneOffset := 0
	var llmSettings domain.LlmSettings
	if result := db.First(&llmSettings); result.Error == nil {
		tagScanEnabled = llmSettings.TagScanEnabled
		tagScanStartHour = llmSettings.TagScanStartHour
		tagScanStartMinute = llmSettings.TagScanStartMinute
		tagScanEndHour = llmSettings.TagScanEndHour
		tagScanEndMinute = llmSettings.TagScanEndMinute
		tagScanTimezoneOffset = llmSettings.TagScanTimezoneOffset
	}

	tagScanManager.Start(tagScanEnabled, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset)
	defer tagScanManager.Stop()

	// Set coordinator for AI task synchronization
	llmOcrService.SetCoordinator(tagScanManager)

	fmt.Printf("Tag scan: window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v\n", tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset, tagScanEnabled)

	// Create embedding backfill manager
	embeddingBackfill := imaging.NewEmbeddingBackfillManager(db)
	fmt.Println("Embedding backfill manager initialized")

	// Create MCP server
	llmFactory := helpers.NewLLMFactory(db, cfg.LlmMaxImageMegapixels)
	mcpSrv := mcpserver.NewImageToolkitMCPServer(db, llmFactory, llmOcrService, cfg.LlmMaxImageMegapixels)
	fmt.Println("MCP server initialized with image analysis and search tools")

	// Create conversation service and agent
	convService := agentpkg.NewConversationService(db)
	agCfg := agentpkg.DefaultAgentConfig()
	agCfg.MaxConversationTokens = cfg.AgentMaxConversationTokens
	ag := agentpkg.NewAgent(convService, mcpSrv, agCfg)
	fmt.Println("AI agent initialized")

	// Start web server
	server := handler.NewServer(db, scanManager, ocrManager, llmOcrService, backgroundSync, tagScanManager, embeddingBackfill, thumbnailService, cfg, geolocationService, nominatimClient, mcpSrv, ag, agCfg, convService)
	router := server.SetupRouter(authMiddleware, csrfProtection, authHandlers)

	// Start OCR health check if enabled
	server.StartOCRHealthCheck()
	defer server.StopOCRHealthCheck()

	fmt.Printf("\nStarting API server on http://%s:%s\n", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("Scan workers: %d\n", cfg.ScanWorkers)
	fmt.Printf("CORS allowed origins: %s\n", strings.Join(cfg.CORSOrigins, ", "))
	fmt.Printf("Thumbnail cache: enabled=%v, path=%s\n", cfg.ThumbnailCacheEnabled, cachePath)
	fmt.Printf("Background sync: daily at %02d:%02d, enabled=%v (configured in UI)\n", syncHour, syncMinute, syncEnabled)
	fmt.Println("Configure gallery folders via the web UI Settings tab.")
	fmt.Println("Press Ctrl+C to stop the server")

	if err := router.Run(fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
