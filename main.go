package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func main() {
	config := apiConfig{
		fileserverHits: atomic.Int32{},
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
		config.fileserverHits.Swap(0)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/healthz", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Content-Type", "text/plain; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("OK"))
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

			errResponse, _ := json.Marshal(ApiError{
				Error: "Something went wrong",
			})

			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write(errResponse)
			return
		}
		defer request.Body.Close()

		if len(reqBody.Body) > 140 {
			fmt.Println("Request body too long")

			errResponse, _ := json.Marshal(ApiError{
				Error: "Chirp is too long",
			})
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write(errResponse)
			return
		}

		type validResponse struct {
			Valid bool `json:"valid"`
		}

		resData, _ := json.Marshal(validResponse{
			Valid: true,
		})

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(resData)
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
