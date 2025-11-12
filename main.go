package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/vladnkolesnikov/chirpy/internal/app"
	"github.com/vladnkolesnikov/chirpy/internal/auth"
	"github.com/vladnkolesnikov/chirpy/internal/database"
	"github.com/vladnkolesnikov/chirpy/internal/utils"

	"net/http"
)

func main() {
	appConfig := app.New()

	defer func() {
		err := appConfig.DB.Close()
		if err != nil {
			log.Fatalf("Error closing database connection: %v", err)
		}
	}()

	mux := http.NewServeMux()
	fileHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	mux.Handle("/app/", appConfig.MiddlewareMetricsInc(fileHandler))

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		hits := int(appConfig.FileserverHits.Load())

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
		if appConfig.ENV != "dev" {
			utils.RespondWithError(w, http.StatusForbidden, "Forbidden")
			return
		}

		err := appConfig.Queries.DeleteAllUsers(r.Context())

		if err != nil {
			fmt.Printf("Error deleting users: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error deleting users")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, "Ok")
	})

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		err := appConfig.DB.Ping()
		if err != nil {
			fmt.Printf("Error pinging database: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		type response struct {
			Status string `json:"status"`
		}

		utils.RespondWithJSON(w, http.StatusOK, response{
			Status: "ok",
		})
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type requestPayload struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			fmt.Println("Error decoding body:", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		defer r.Body.Close()

		hash, err := auth.HashPassword(reqBody.Password)
		if err != nil {
			fmt.Printf("Error hashing password: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		user, err := appConfig.Queries.CreateUser(r.Context(), database.CreateUserParams{
			Email:    reqBody.Email,
			Password: hash,
		})

		if err != nil {
			fmt.Println("Error creating user:", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type requestPayload struct {
			Password         string `json:"password"`
			Email            string `json:"email"`
			ExpiresInSeconds *uint8 `json:"expires_in_seconds,omitempty"`
		}

		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			fmt.Printf("Error decoding request body: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		defer r.Body.Close()

		user, err := appConfig.Queries.GetUserByEmail(r.Context(), reqBody.Email)

		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("User with email %s not found\n", reqBody.Email)
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		matches, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)

		if err != nil {
			fmt.Printf("Error matching passwords: %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		if !matches {
			fmt.Println("Invalid password")
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		type response struct {
			database.User
			Token string `json:"token"`
		}

		var expires time.Duration

		if reqBody.ExpiresInSeconds != nil {
			expires = time.Duration(*reqBody.ExpiresInSeconds) * time.Second
		} else {
			expires = time.Hour
		}

		token, err := auth.MakeJWT(user.ID, appConfig.Secret, expires)
		if err != nil {
			fmt.Printf("Error creating token for user with ID: %v\n", user.ID)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, response{
			User:  user,
			Token: token,
		})
	})

	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)

		if err != nil {
			fmt.Printf("Error reading auth token: %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, "Token is missing")
			return
		}

		userID, err := auth.ValidateJWT(token, appConfig.Secret)
		if err != nil {
			fmt.Printf("Error validating JWT %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, "Something went wrong")
			return
		}

		type requestPayload struct {
			Body string `json:"body"`
		}
		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			fmt.Printf("Error decoding request body: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		defer r.Body.Close()

		if len(reqBody.Body) > 140 {
			fmt.Println("Request body too long")
			utils.RespondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		chirp, err := appConfig.Queries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   reqBody.Body,
			UserID: userID,
		})

		if err != nil {
			fmt.Printf("Error creating new chirp: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusCreated, chirp)
	})

	mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := appConfig.Queries.GetChirps(r.Context())
		if err != nil {
			fmt.Printf("Error getting chirps: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, chirps)
	})

	mux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpIDParam := r.PathValue("chirpID")
		err := uuid.Validate(chirpIDParam)

		if err != nil {
			fmt.Println("Invalid chirpID uuid")
			utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		chirpID := uuid.MustParse(chirpIDParam)

		chirp, err := appConfig.Queries.GetChirpByID(r.Context(), chirpID)
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("Chirp with ID %v not found\n", chirpID)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		if err != nil {
			fmt.Printf("Error getting chirp by ID %v: %s\n", chirpID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, chirp)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
