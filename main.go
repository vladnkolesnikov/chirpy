package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/vladnkolesnikov/chirpy/internal/database"

	"net/http"
	"slices"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
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

func hideWords(body string) string {
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}
	strs := strings.Split(body, " ")

	for index, word := range strs {
		if slices.Contains(profaneWords, strings.ToLower(word)) == true {
			strs[index] = "****"
		}
	}

	return strings.Join(strs, " ")
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
	w.WriteHeader(code)
	w.Write(resData)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	db, err := sql.Open("pgx", dbURL)
	dbQueries := database.New(db)

	config := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
		platform:       platform,
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
		if config.platform != "dev" {
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

	mux.HandleFunc("POST /api/validate_chirp", func(writer http.ResponseWriter, request *http.Request) {
		type requestBody struct {
			Body string `json:"body"`
		}
		reqBody := &requestBody{}

		decoder := json.NewDecoder(request.Body)
		err := decoder.Decode(reqBody)

		if err != nil {
			fmt.Printf("Error decoding request body: %s\n", err)
			respondWithError(writer, http.StatusInternalServerError, "Something went wrong")
			return
		}
		defer request.Body.Close()

		if len(reqBody.Body) > 140 {
			fmt.Println("Request body too long")
			respondWithError(writer, http.StatusBadRequest, "Chirp is too long")
			return
		}

		type validResponse struct {
			CleanedBody string `json:"cleaned_body"`
		}

		respondWithJSON(writer, http.StatusOK, validResponse{
			CleanedBody: hideWords(reqBody.Body),
		})
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
