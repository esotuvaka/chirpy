package main

import (
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
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
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
		w.WriteHeader(403)
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

func handlerValidate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
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
		w.WriteHeader(400)
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
		w.WriteHeader(400)
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

		response := struct {
			CleanedBody string `json:"cleaned_body"`
		}{
			CleanedBody: cleanedBody,
		}

		w.WriteHeader(200)
		json.NewEncoder(w).Encode(response)
	}
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}
	type errorBody struct {
		Err string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
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

	user, err := cfg.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
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

	w.WriteHeader(201)
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
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
		platform:       os.Getenv("PLATFORM"),
	}

	logger, err := initLogger()
	if err != nil {
		panic("initializing logger")
	}

	server := NewServer(cfg, *logger)
	server.router.Handle("GET /app/", apiCfg.middlewareMetrics(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	server.router.Handle("GET /assets", http.FileServer(http.Dir("./assets")))
	server.router.Handle("GET /api/healthz", http.HandlerFunc(handlerHealth))
	server.router.Handle("POST /api/validate-chirp", http.HandlerFunc(handlerValidate))
	server.router.Handle("POST /api/users", http.HandlerFunc(apiCfg.handlerCreateUser))
	server.router.Handle("POST /admin/reset", http.HandlerFunc(apiCfg.handlerReset))
	server.router.Handle("GET /admin/metrics", http.HandlerFunc(apiCfg.handlerHits))

	if err := server.Start(); err != nil {
		logger.Fatal("starting server")
	}
}
