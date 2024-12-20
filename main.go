package main

import (
	"chirpy/internal/api"
	"chirpy/internal/database"
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
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
	apiCfg := api.Config{
		FileserverHits:   atomic.Int32{},
		DbQueries:        dbQueries,
		Platform:         os.Getenv("PLATFORM"),
		JwtSigningSecret: os.Getenv("JWT_SIGNING_KEY"),
	}

	logger, err := initLogger()
	if err != nil {
		panic("initializing logger")
	}

	server := NewServer(cfg, *logger)
	server.router.Handle("GET /app/", apiCfg.MiddlewareMetrics(http.StripPrefix(
		"/app", http.FileServer(http.Dir(".")))))
	server.router.Handle("GET /assets", http.FileServer(http.Dir("./assets")))
	server.router.Handle("GET /api/healthz", http.HandlerFunc(handlerHealth))
	server.router.Handle("POST /api/users", http.HandlerFunc(apiCfg.CreateUser))
	server.router.Handle("PUT /api/users", http.HandlerFunc(apiCfg.UpdateUserLogin))
	server.router.Handle("POST /api/login", http.HandlerFunc(apiCfg.Login))
	server.router.Handle("POST /api/refresh", http.HandlerFunc(apiCfg.Refresh))
	server.router.Handle("POST /api/revoke", http.HandlerFunc(apiCfg.Revoke))
	server.router.Handle("POST /api/chirps", http.HandlerFunc(apiCfg.CreateChirp))
	server.router.Handle("GET /api/chirps", http.HandlerFunc(apiCfg.ListChirps))
	server.router.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(apiCfg.GetChirp))
	server.router.Handle("DELETE /api/chirps/{chirpID}", http.HandlerFunc(apiCfg.DeleteChirp))
	server.router.Handle("POST /api/polka/webhooks", http.HandlerFunc(apiCfg.UpgradeUser))
	server.router.Handle("POST /admin/reset", http.HandlerFunc(apiCfg.ResetHitsAndUsers))
	server.router.Handle("GET /admin/metrics", http.HandlerFunc(apiCfg.PageHits))

	if err := server.Start(); err != nil {
		logger.Fatal("starting server")
	}
}
