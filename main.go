package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type Config struct {
	ListenAddr     string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
}

type Server struct {
	Config
	router *http.ServeMux
	logger *zap.SugaredLogger
}

func initLogger() (*zap.SugaredLogger, error) {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := config.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

func NewServer(cfg Config, logger zap.SugaredLogger) *Server {
	return &Server{
		Config: cfg,
		router: http.NewServeMux(),
		logger: &logger,
	}
}

func (s *Server) Start() error {
	server := &http.Server{
		Addr:           s.ListenAddr,
		Handler:        s.router,
		ReadTimeout:    s.ReadTimeout,
		WriteTimeout:   s.WriteTimeout,
		IdleTimeout:    s.IdleTimeout,
		MaxHeaderBytes: s.MaxHeaderBytes,
	}

	go func() {
		s.logger.Info("Starting server on port ", s.ListenAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Fatal("Server failed: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileserverHits   atomic.Int32
	dbQueries        *database.Queries
	platform         string
	jwtSigningSecret string
}

func (cfg *apiConfig) handlerHits(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf(`<html>
            <body>
                <h1>Welcome, Chirpy Admin</h1>
                <p>Chirpy has been visited %d times!</p>
            </body>
        </html>`, cfg.fileserverHits.Load())
	w.Write([]byte(resp))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.dbQueries.DeleteAllUsers(r.Context())
	cfg.fileserverHits.Swap(0)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error hashing password: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	user, err := cfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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

	response := struct {
		Id        string `json:"id"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Email     string `json:"email"`
	}{
		Id:        user.ID.String(),
		CreatedAt: user.CreatedAt.String(),
		UpdatedAt: user.UpdatedAt.String(),
		Email:     user.Email,
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (cfg *apiConfig) handlerLoginUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password         string  `json:"password"`
		Email            string  `json:"email"`
		ExpiresInSeconds *string `json:"expires_in_seconds"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
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

	user, err := cfg.dbQueries.FindUserByEmail(r.Context(), params.Email)
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

	response := struct {
		Id        string `json:"id"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Email     string `json:"email"`
	}{
		Id:        user.ID.String(),
		CreatedAt: user.CreatedAt.String(),
		UpdatedAt: user.UpdatedAt.String(),
		Email:     user.Email,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
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

		chirp, err := cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
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
			CreatedAt: chirp.CreatedAt.String(),
			UpdatedAt: chirp.UpdatedAt.String(),
			Body:      chirp.Body,
			UserId:    chirp.UserID.UUID,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func (cfg *apiConfig) handlerListChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.ListChirps(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := struct {
		Chirps []database.Chirp `json:"items"`
	}{
		Chirps: chirps,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), chirpUUID)
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
		CreatedAt: chirp.CreatedAt.String(),
		UpdatedAt: chirp.UpdatedAt.String(),
		Body:      chirp.Body,
		UserId:    chirp.UserID.UUID.String(),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic("intializing postgres db connection")
	}
	dbQueries := database.New(db)

	cfg := Config{
		ListenAddr:     ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1mb
	}
	apiCfg := apiConfig{
		fileserverHits:   atomic.Int32{},
		dbQueries:        dbQueries,
		platform:         os.Getenv("PLATFORM"),
		jwtSigningSecret: os.Getenv("JWT_SIGNING_KEY"),
	}

	logger, err := initLogger()
	if err != nil {
		panic("initializing logger")
	}

	server := NewServer(cfg, *logger)
	server.router.Handle("GET /app/", apiCfg.middlewareMetrics(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	server.router.Handle("GET /assets", http.FileServer(http.Dir("./assets")))
	server.router.Handle("GET /api/healthz", http.HandlerFunc(handlerHealth))
	server.router.Handle("POST /api/users", http.HandlerFunc(apiCfg.handlerCreateUser))
	server.router.Handle("POST /api/login", http.HandlerFunc(apiCfg.handlerLoginUser))
	server.router.Handle("POST /api/chirps", http.HandlerFunc(apiCfg.handlerCreateChirp))
	server.router.Handle("GET /api/chirps", http.HandlerFunc(apiCfg.handlerListChirps))
	server.router.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(apiCfg.handlerGetChirp))
	server.router.Handle("POST /admin/reset", http.HandlerFunc(apiCfg.handlerReset))
	server.router.Handle("GET /admin/metrics", http.HandlerFunc(apiCfg.handlerHits))

	if err := server.Start(); err != nil {
		logger.Fatal("starting server")
	}
}
