package api

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (cfg *Config) CreateChirp(w http.ResponseWriter, r *http.Request) {
	// validate auth before processing any further
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("extracting bearer token from header: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	userId, err := auth.ValidateJWT(token, os.Getenv("JWT_SIGNING_KEY"))
	if err != nil {
		log.Printf("validating token: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	// process the request
	type parameters struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		errBody := errorBody{
			Err: "Something went wrong",
		}
		eBody, err := json.Marshal(errBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.Write(eBody)
	}

	if userId.String() != params.UserId {
		log.Printf("WARNING: User '%s' is requesting User '%s' resources", userId, params.UserId)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("FORBIDDEN"))
		return
	}

	if len(params.Body) > 140 {
		w.WriteHeader(http.StatusBadRequest)
		errBody := errorBody{
			Err: "Chirp is too long",
		}
		eBody, err := json.Marshal(errBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.Write(eBody)
	} else if len(params.Body) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		errBody := errorBody{
			Err: "Request JSON should be in shape {'body': 'chirp message...'}",
		}
		eBody, err := json.Marshal(errBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			return
		}
		w.Write(eBody)
	} else {
		profaneWords := []string{"kerfuffle", "sharbert", "fornax"}
		cleanedBody := strings.ToLower(params.Body)

		for _, word := range profaneWords {
			pattern := regexp.MustCompile(`(?i)\b` + word + `\b`)
			cleanedBody = pattern.ReplaceAllString(cleanedBody, "****")
		}

		userUUID, err := uuid.Parse(params.UserId)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		chirp, err := cfg.DbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
			Body: cleanedBody,
			UserID: uuid.NullUUID{
				UUID:  userUUID,
				Valid: true,
			},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := struct {
			Id        uuid.UUID `json:"id"`
			CreatedAt string    `json:"created_at"`
			UpdatedAt string    `json:"updated_at"`
			Body      string    `json:"body"`
			UserId    uuid.UUID `json:"user_id"`
		}{
			Id:        userUUID,
			CreatedAt: chirp.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: chirp.UpdatedAt.UTC().Format(time.RFC3339),
			Body:      chirp.Body,
			UserId:    chirp.UserID.UUID,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func (cfg *Config) ListChirps(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	authorId := queryValues.Get("author_id")
	authorUUID, err := uuid.Parse(authorId)
	if err != nil {
		log.Printf("invalid author id: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	var chirps []database.Chirp
	if authorId != "" {
		chirpsList, err := cfg.DbQueries.ListChirpsByAuthor(r.Context(), uuid.NullUUID{
			UUID:  authorUUID,
			Valid: true,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		chirps = chirpsList
	} else {
		chirpsList, err := cfg.DbQueries.ListChirps(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		chirps = chirpsList
	}

	response := struct {
		Chirps []database.Chirp `json:"items"`
	}{
		Chirps: chirps,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *Config) GetChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chirp, err := cfg.DbQueries.GetChirp(r.Context(), chirpUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := struct {
		Id        string `json:"id"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Body      string `json:"body"`
		UserId    string `json:"user_id"`
	}{
		Id:        chirp.ID.String(),
		CreatedAt: chirp.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: chirp.UpdatedAt.UTC().Format(time.RFC3339),
		Body:      chirp.Body,
		UserId:    chirp.UserID.UUID.String(),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *Config) DeleteChirp(w http.ResponseWriter, r *http.Request) {
	// validate auth before processing any further
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("extracting bearer token from header: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	userId, err := auth.ValidateJWT(token, os.Getenv("JWT_SIGNING_KEY"))
	if err != nil {
		log.Printf("validating token: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("UNAUTHORIZED"))
		return
	}

	chirpID := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		log.Printf("bad chirp id")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("BAD REQUEST"))
		return
	}

	chirp, err := cfg.DbQueries.GetChirp(r.Context(), chirpUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("chirp not found")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("NOT FOUND"))
			return
		}
		log.Printf("finding chirp: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	if chirp.UserID.UUID != userId {
		log.Printf("user '%s' requested data for user '%s'", userId, chirp.UserID.UUID)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("FORBIDDEN"))
		return
	}

	err = cfg.DbQueries.DeleteChirp(r.Context(), chirpUUID)
	if err != nil {
		log.Printf("deleting chirp in db: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("INTERNAL SERVER ERROR"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
