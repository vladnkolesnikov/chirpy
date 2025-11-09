package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/vladnkolesnikov/chirpy/internal/database"

	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	env            string
}

func (apiConfig *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiConfig.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

type ApiError struct {
	Error string `json:"error"`
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	errResponse, _ := json.Marshal(ApiError{
		Error: msg,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(errResponse)
}

func respondWithJSON[T any](w http.ResponseWriter, code int, payload T) {
	resData, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	if code != http.StatusOK {
		w.WriteHeader(code)
	}

	w.Write(resData)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	env := os.Getenv("PLATFORM")

	db, err := sql.Open("pgx", dbURL)

	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	defer func() {
		err := db.Close()
		if err != nil {
			log.Fatalf("Error closing database connection: %v", err)
		}
	}()

	dbQueries := database.New(db)

	config := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
		env:            env,
	}

	mux := http.NewServeMux()

	fileHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	mux.Handle("/app/", config.middlewareMetricsInc(fileHandler))

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		hits := int(config.fileserverHits.Load())

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		template := fmt.Sprintf(`
		<html>
	  		<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
	  		</body>
		</html>`, hits)

		w.Write([]byte(template))
	})

	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if config.env != "dev" {
			respondWithError(w, http.StatusForbidden, "Forbidden")
			return
		}

		err := config.dbQueries.DeleteAllUsers(r.Context())

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error deleting users")
			return
		}

		respondWithJSON(w, http.StatusOK, "Ok")
	})

	mux.HandleFunc("GET /api/healthz", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Content-Type", "text/plain; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Email string `json:"email"`
		}
		reqBody := &requestBody{}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(reqBody)

		if err != nil {
			fmt.Println("Error decoding body:", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		defer r.Body.Close()

		user, err := config.dbQueries.CreateUser(r.Context(), reqBody.Email)

		if err != nil {
			fmt.Println("Error creating user:", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		respondWithJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Body   string `json:"body"`
			UserId string `json:"user_id"`
		}
		reqBody := &requestBody{}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(reqBody)

		if err != nil {
			fmt.Printf("Error decoding r body: %s\n", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		defer r.Body.Close()

		if len(reqBody.Body) > 140 {
			fmt.Println("Request body too long")
			respondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		if err := uuid.Validate(reqBody.UserId); err != nil {
			fmt.Println("Invalid userId UUID")
			respondWithError(w, http.StatusBadRequest, "Invalid userId")
			return
		}

		chirp, err := config.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   reqBody.Body,
			UserID: uuid.MustParse(reqBody.UserId),
		})

		if err != nil {
			fmt.Printf("Error creating new chirp: %s\n", err)
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("something went wrong: %v", err))
			return
		}

		respondWithJSON(w, http.StatusCreated, chirp)
	})

	mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := config.dbQueries.GetChirps(r.Context())
		if err != nil {
			fmt.Printf("Error getting chirps: %s\n", err)
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		respondWithJSON(w, http.StatusOK, chirps)
	})

	mux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpIDParam := r.PathValue("chirpID")
		err := uuid.Validate(chirpIDParam)

		if err != nil {
			fmt.Println("Invalid chirpID uuid")
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		chirpID := uuid.MustParse(chirpIDParam)

		chirp, err := config.dbQueries.GetChirp(r.Context(), chirpID)
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("Chirp with ID %v not found\n", chirpID)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		if err != nil {
			fmt.Printf("Error getting chirp by ID %v: %s\n", chirpID, err)
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		respondWithJSON(w, http.StatusOK, chirp)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
