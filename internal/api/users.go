package api

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func (cfg *Config) CreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	user, err := cfg.DbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Printf("creating user in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	response := struct {
		Id          string `json:"id"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		Email       string `json:"email"`
		IsChirpyRed bool   `json:"is_chirpy_red"`
	}{
		Id:          user.ID.String(),
		CreatedAt:   user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   user.UpdatedAt.UTC().Format(time.RFC3339),
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (cfg *Config) UpdateUserLogin(w http.ResponseWriter, r *http.Request) {
	accessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("extracting auth header")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	userId, err := auth.ValidateJWT(accessToken, os.Getenv("JWT_SIGNING_KEY"))
	if err != nil {
		log.Printf("validating token: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("hashing password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	user, err := cfg.DbQueries.UpdateUserLogin(r.Context(), database.UpdateUserLoginParams{
		ID:             userId,
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Printf("updating user login in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	response := struct {
		Id          string `json:"id"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		Email       string `json:"email"`
		IsChirpyRed bool   `json:"is_chirpy_red"`
	}{
		Id:          user.ID.String(),
		CreatedAt:   user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   user.UpdatedAt.UTC().Format(time.RFC3339),
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
