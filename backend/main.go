package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"sensor-backend/internal/db"
	"sensor-backend/internal/handlers"
	"sensor-backend/internal/middleware"
	"sensor-backend/internal/schema"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env not loaded: %v", err)
	}
	schemaPath := "../schema.yaml"
	if p := os.Getenv("SCHEMA_PATH"); p != "" {
		schemaPath = p
	}
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("read schema: %v", err)
	}
	var s schema.Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		log.Fatalf("parse schema: %v", err)
	}
	if s.TableName == "" {
		s.TableName = "sensor_metrics"
	}

	connStr := "host=" + getEnv("DB_HOST", "localhost") +
		" port=" + getEnv("DB_PORT", "5432") +
		" user=" + getEnv("DB_USER", "sensor") +
		" password=" + getEnv("DB_PASSWORD", "sensor_secret") +
		" dbname=" + getEnv("DB_NAME", "sensor_db") +
		" sslmode=" + getEnv("DB_SSLMODE", "disable")
	database, err := db.InitDB(connStr, &s)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer database.Close()

	readKey := os.Getenv("READ_API_KEY")
	writeKey := os.Getenv("WRITE_API_KEY")
	if readKey == "" || writeKey == "" {
		log.Fatal("READ_API_KEY and WRITE_API_KEY must be set")
	}

	mux := http.NewServeMux()
    
    // 1. Remove CORS from the individual routes. Leave Auth wrapping the Handlers.
    mux.Handle("GET /schema", middleware.Auth(readKey, writeKey, handlers.SchemaHandler(&s)))
    mux.Handle("GET /metrics", middleware.Auth(readKey, writeKey, handlers.MetricsHandler(database, &s)))
    mux.Handle("POST /ingest", middleware.Auth(readKey, writeKey, handlers.IngestHandler(database, &s)))

    // 2. Wrap the ENTIRE router with the CORS middleware
    corsHandler := middleware.CORS(mux)

    addr := ":" + getEnv("PORT", "8080")
    log.Printf("listening on %s", addr)
    
    // 3. Pass the wrapped corsHandler to the server
    if err := http.ListenAndServe(addr, corsHandler); err != nil {
        log.Fatal(err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
