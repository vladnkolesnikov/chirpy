package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

type ApiError struct {
	Error string `json:"error"`
}

func RespondWithError(w http.ResponseWriter, code int, msg string) {
	errResponse, _ := json.Marshal(ApiError{
		Error: msg,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(errResponse)
}

func RespondWithJSON[T any](w http.ResponseWriter, code int, payload T) {
	w.Header().Set("Content-Type", "application/json")
	resData, err := json.Marshal(payload)

	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)
	w.Write(resData)
}

func DecodeBody[T any](r *http.Request, body T) (T, error) {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&body)
	return body, err
}
