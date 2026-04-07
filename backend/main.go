package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// init is invoked before main()
func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Parse command line arguments
	port := flag.String("port", config.ServerPort, "HTTP server port")
	flag.Parse()

	// Get directories from remaining arguments
	dirs := flag.Args()
	if len(dirs) == 0 {
		fmt.Println("Usage: image-dedup [options] <directory1> [directory2] ...")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nEnvironment variables:")
		fmt.Println("  DB_HOST      PostgreSQL host (default: localhost)")
		fmt.Println("  DB_PORT      PostgreSQL port (default: 5432)")
		fmt.Println("  DB_USER      PostgreSQL user (default: postgres)")
		fmt.Println("  DB_PASSWORD  PostgreSQL password (default: postgres)")
		fmt.Println("  DB_NAME      Database name (default: image_dedup)")
		fmt.Println("  SERVER_PORT  HTTP server port (default: 8080)")
		fmt.Println("  CORS_ORIGINS Comma-separated allowed origins (default: http://localhost:5173)")
		os.Exit(1)
	}

	// Validate directories
	var validDirs []string
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			log.Printf("Warning: Cannot access directory %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			log.Printf("Warning: %s is not a directory", dir)
			continue
		}
		validDirs = append(validDirs, dir)
	}

	if len(validDirs) == 0 {
		log.Fatal("No valid directories provided")
	}

	fmt.Printf("Image Dedup - API Server\n")
	fmt.Printf("========================\n\n")

	// Initialize database
	fmt.Println("Connecting to PostgreSQL database...")
	db, err := initDatabase(config)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	fmt.Println("Database connected successfully!")

	// Initial scan
	fmt.Printf("\nScanning directories: %s\n", strings.Join(validDirs, ", "))
	progressChan := make(chan string, 100)

	go func() {
		for msg := range progressChan {
			fmt.Printf("  %s\n", msg)
		}
	}()

	cleanupMissingFiles(db, progressChan)

	for _, dir := range validDirs {
		fmt.Printf("\nScanning: %s\n", dir)
		if err := scanDirectory(db, dir, progressChan); err != nil {
			log.Printf("Error scanning %s: %v", dir, err)
		}
	}
	close(progressChan)

	groups, _ := findDuplicates(db)
	fmt.Printf("\n========================\n")
	fmt.Printf("Scan complete! Found %d duplicate groups.\n", len(groups))

	// Create scan manager for async rescans
	scanManager := NewScanManager(db, validDirs)

	// Update port from CLI flag if provided
	config.ServerPort = *port

	// Start web server
	server := NewServer(db, validDirs, scanManager, config)
	router := server.SetupRouter()

	fmt.Printf("\nStarting API server on http://localhost:%s\n", *port)
	fmt.Printf("CORS allowed origins: %s\n", strings.Join(config.CORSOrigins, ", "))
	fmt.Println("Press Ctrl+C to stop the server")

	if err := router.Run(fmt.Sprintf(":%s", *port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
