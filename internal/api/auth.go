package api

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

func (cfg *Config) Login(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request"))
		return
	}

	user, err := cfg.DbQueries.FindUserByEmail(r.Context(), params.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Incorrect email or password"))
			return
		}
		log.Printf("Error while searching user by email")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		log.Printf("Error while checking password hash equivalence")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Incorrect email or password"))
		return
	}

	expires := 3600 // 1hr
	exp := time.Duration(expires) * time.Second
	token, err := auth.MakeJWT(user.ID, os.Getenv("JWT_SIGNING_KEY"), exp)
	if err != nil {
		log.Printf("creating JWT for user: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		log.Printf("creating refresh token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	daysAsHours := 60 * 24
	refreshExpires := time.Duration(daysAsHours) * time.Hour
	err = cfg.DbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token: refreshToken,
		UserID: uuid.NullUUID{
			UUID:  user.ID,
			Valid: true,
		},
		ExpiresAt: time.Now().Add(refreshExpires).UTC(),
	})
	if err != nil {
		log.Printf("storing refresh token: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	response := struct {
		Id           string `json:"id"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		Email        string `json:"email"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}{
		Id:           user.ID.String(),
		CreatedAt:    user.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    user.UpdatedAt.UTC().Format(time.RFC3339),
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *Config) Refresh(w http.ResponseWriter, r *http.Request) {
	// get the refresh token
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("decoding parameters: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	// find the refresh token in DB
	refresh, err := cfg.DbQueries.FindRefreshToken(r.Context(), refreshToken)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("refresh token not found")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("UNAUTHORIZED"))
			return
		}
		log.Printf("finding refresh token in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}
	if time.Now().After(refresh.ExpiresAt) {
		log.Printf("refresh token expired")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("TOKEN EXPIRED"))
		return
	}

	user, err := cfg.DbQueries.FindUserById(r.Context(), refresh.UserID.UUID)
	if err != nil {
		log.Printf("finding user by id")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("USER NOT FOUND"))
		return
	}

	expires := 3600 // 1hr
	exp := time.Duration(expires) * time.Second
	token, err := auth.MakeJWT(user.ID, os.Getenv("JWT_SIGNING_KEY"), exp)
	if err != nil {
		log.Printf("creating JWT for user: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	response := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *Config) Revoke(w http.ResponseWriter, r *http.Request) {
	// get the refresh token
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("decoding parameters: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	// find the refresh token in DB
	refresh, err := cfg.DbQueries.FindRefreshToken(r.Context(), refreshToken)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("refresh token not found")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("UNAUTHORIZED"))
			return
		}
		log.Printf("finding refresh token in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}
	if time.Now().After(refresh.ExpiresAt) {
		log.Printf("refresh token expired")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("TOKEN EXPIRED"))
		return
	}

	err = cfg.DbQueries.UpdateRefreshToken(r.Context(), refreshToken)
	if err != nil {
		log.Printf("updating refresh token in db")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
