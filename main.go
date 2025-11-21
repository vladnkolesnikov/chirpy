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
			log.Printf("Error deleting users: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error deleting users")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, "Ok")
	})

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		err := appConfig.DB.Ping()
		if err != nil {
			log.Printf("Error pinging database: %s\n", err)
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
			log.Println("Error decoding body:", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		defer r.Body.Close()

		hash, err := auth.HashPassword(reqBody.Password)
		if err != nil {
			log.Printf("Error hashing password: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		user, err := appConfig.Queries.CreateUser(r.Context(), database.CreateUserParams{
			Email:    reqBody.Email,
			Password: hash,
		})

		if err != nil {
			log.Println("Error creating user:", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusCreated, user)
	})

	mux.HandleFunc("PUT /api/users", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)

		if err != nil {
			log.Printf("[Error reading auth token]: %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}

		userID, err := auth.ValidateJWT(token, appConfig.Secret)
		if err != nil {
			log.Printf("[Error validating JWT] %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}

		type requestPayload struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			log.Println("Error decoding body:", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		defer r.Body.Close()

		hash, err := auth.HashPassword(reqBody.Password)
		if err != nil {
			log.Printf("Error hashing password: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		updatedUser, err := appConfig.Queries.UpdateUserInfo(r.Context(), database.UpdateUserInfoParams{
			ID:             userID,
			Email:          reqBody.Email,
			HashedPassword: hash,
		})

		if err != nil {
			log.Printf("Error updating user %s: %s\n", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, updatedUser)
	})

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type requestPayload struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			log.Printf("Error decoding request body: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		defer r.Body.Close()

		user, err := appConfig.Queries.GetUserByEmail(r.Context(), reqBody.Email)

		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("User with email %s not found\n", reqBody.Email)
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		matches, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)

		if err != nil {
			log.Printf("Error matching passwords: %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		if !matches {
			log.Println("Invalid password")
			utils.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		type response struct {
			database.User
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		}

		token, err := auth.MakeJWT(user.ID, appConfig.Secret, time.Hour)
		if err != nil {
			log.Printf("Error creating token for user with ID: %v\n", user.ID)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		refreshToken, err := auth.MakeRefreshToken()
		if err != nil {
			log.Printf("Error creating refresh token: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		if err = appConfig.Queries.CreateToken(r.Context(), database.CreateTokenParams{
			Token:     refreshToken,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(time.Hour * 24 * 60),
		}); err != nil {
			log.Printf("Error saving refresh token: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, response{
			User:         user,
			Token:        token,
			RefreshToken: refreshToken,
		})
	})

	mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		authToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error reading auth header: %s\n", err)
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid authentication token")
			return
		}

		tokenData, err := appConfig.Queries.GetToken(r.Context(), authToken)
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Token not found")
			utils.RespondWithError(w, http.StatusUnauthorized, "Invalid authorization token")
			return
		}

		if err != nil {
			log.Printf("Error reading refresh token from db: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Invalid authorization token")
			return
		}

		if time.Now().After(tokenData.ExpiresAt) || tokenData.RevokedAt.Valid == true {
			log.Printf("Expired token")
			utils.RespondWithError(w, http.StatusUnauthorized, "Invalid authorization token")
			return
		}

		type response struct {
			Token string `json:"token"`
		}

		jwtToken, err := auth.MakeJWT(tokenData.UserID, appConfig.Secret, time.Hour)
		if err != nil {
			log.Printf("Error creating token for user with ID: %v\n", tokenData.UserID)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error creating jwt")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, response{
			Token: jwtToken,
		})
	})

	mux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, r *http.Request) {
		authToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error reading auth header: %s\n", err)
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid authentication token")
			return
		}

		if err = appConfig.Queries.RevokeToken(r.Context(), database.RevokeTokenParams{
			Token: authToken,
			RevokedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			UpdatedAt: time.Now(),
		}); err != nil {
			log.Printf("Error reading auth header: %s\n", err)
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid authentication token")
			return
		}

		utils.RespondWithJSON(w, http.StatusNoContent, struct{}{})
	})

	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)

		if err != nil {
			log.Printf("[Error reading auth token]: %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}

		userID, err := auth.ValidateJWT(token, appConfig.Secret)
		if err != nil {
			log.Printf("[Error validating JWT] %s\n", err)
			utils.RespondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}

		type requestPayload struct {
			Body string `json:"body"`
		}
		reqBody, err := utils.DecodeBody(r, requestPayload{})

		if err != nil {
			log.Printf("[Error decoding request body]: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		defer r.Body.Close()

		if len(reqBody.Body) > 140 {
			log.Println("Request body max size exceeded")
			utils.RespondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		chirp, err := appConfig.Queries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body:   reqBody.Body,
			UserID: userID,
		})

		if err != nil {
			log.Printf("Error creating new chirp: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusCreated, chirp)
	})

	mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := appConfig.Queries.GetChirps(r.Context())
		if err != nil {
			log.Printf("Error getting chirps: %s\n", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, chirps)
	})

	mux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, r *http.Request) {
		chirpIDParam := r.PathValue("chirpID")
		err := uuid.Validate(chirpIDParam)

		if err != nil {
			log.Println("Invalid chirpID uuid")
			utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
			return
		}

		chirpID := uuid.MustParse(chirpIDParam)

		chirp, err := appConfig.Queries.GetChirpByID(r.Context(), chirpID)
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Chirp with ID %v not found\n", chirpID)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		if err != nil {
			log.Printf("Error getting chirp by ID %v: %s\n", chirpID, err)
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
