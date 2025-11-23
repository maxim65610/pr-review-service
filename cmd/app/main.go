package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"pr-review-service/internal/db"
	"pr-review-service/internal/httpapi"
	"pr-review-service/internal/repo"
	"pr-review-service/internal/service"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/prservice?sslmode=disable"
	}

	dbConn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("failed to open db:", err)
	}
	defer dbConn.Close()

	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(time.Hour)

	if err := db.Migrate(dbConn); err != nil {
		log.Fatal("migration failed:", err)
	}

	repository := repo.NewPostgresRepo(dbConn)
	svc := service.NewService(repository)
	h := httpapi.NewHandler(svc)

	log.Println("service started on :8080")
	if err := http.ListenAndServe(":8080", h.Router()); err != nil {
		log.Fatal(err)
	}
}
