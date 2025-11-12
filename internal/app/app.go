package app

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/vladnkolesnikov/chirpy/internal/database"
)

type Config struct {
	FileserverHits atomic.Int32
	Queries        *database.Queries
	DB             *sql.DB
	ENV            string
	Secret         string
}

func (appConfig *Config) MiddlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appConfig.FileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func New() *Config {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	env := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")

	db, err := sql.Open("pgx", dbURL)

	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	dbQueries := database.New(db)

	return &Config{
		FileserverHits: atomic.Int32{},
		Queries:        dbQueries,
		DB:             db,
		ENV:            env,
		Secret:         secret,
	}
}
