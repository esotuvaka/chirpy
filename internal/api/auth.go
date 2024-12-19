package api

import (
	"chirpy/internal/auth"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

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
