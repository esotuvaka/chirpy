package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

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
}

func (cfg *apiConfig) handlerHits(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())
	w.Write([]byte(resp))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Swap(0)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := Config{
		ListenAddr:     ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1mb
	}
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	logger, err := initLogger()
	if err != nil {
		panic("initializing logger")
	}

	server := NewServer(cfg, *logger)
	server.router.Handle("GET /app/", apiCfg.middlewareMetrics(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	server.router.Handle("GET /assets", http.FileServer(http.Dir("./assets")))
	server.router.Handle("GET /healthz", http.HandlerFunc(handlerHealth))
	server.router.Handle("GET /metrics", http.HandlerFunc(apiCfg.handlerHits))
	server.router.Handle("GET /reset", http.HandlerFunc(apiCfg.handlerReset))

	if err := server.Start(); err != nil {
		logger.Fatal("starting server")
	}
}
