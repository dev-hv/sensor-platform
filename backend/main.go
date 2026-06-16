package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"sensor-backend/internal/config"
	"sensor-backend/internal/db"
	"sensor-backend/internal/handlers"
	"sensor-backend/internal/middleware"
	"sensor-backend/internal/schema"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env not loaded: %v", err)
	}
	schemaPath := "../infrastructure/schema.yaml"
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

	keys, err := config.GetAPIKeys(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /schema", middleware.Auth(keys.ReadKey, keys.WriteKey, handlers.SchemaHandler(&s)))
	mux.Handle("GET /metrics", middleware.Auth(keys.ReadKey, keys.WriteKey, handlers.MetricsHandler(database, &s)))
	mux.Handle("POST /ingest", middleware.Auth(keys.ReadKey, keys.WriteKey, handlers.IngestHandler(database, &s)))

	corsHandler := middleware.CORS(mux)

	addr := ":" + getEnv("PORT", "8080")
	log.Printf("listening on %s", addr)
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
