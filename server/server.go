package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/crawler"
	mw "github.com/noahschumacher/llm-txt/server/middleware"
)

type Config struct {
	Port        string
	AppEnv      string
	CrawlConfig crawler.Config
}

type Server struct {
	log      *zap.Logger
	cfg      Config
	router   *chi.Mux
	staticFS fs.FS
}

func New(log *zap.Logger, cfg Config, staticFS fs.FS) *Server {
	r := chi.NewRouter()

	// Global middleware — no timeout here; the SSE /generate endpoint is
	// long-running and manages its own context cancellation.
	r.Use(
		mw.NewZapLoggerMiddleware(log),
		middleware.Recoverer,
	)

	return &Server{
		log:      log,
		cfg:      cfg,
		router:   r,
		staticFS: staticFS,
	}
}

func (s *Server) ListenAndServe() error {
	s.log.Sugar().Infof("starting server on port %s", s.cfg.Port)

	// -------------------------------------------------------------------------
	// Health

	s.router.With(mw.TimeoutMiddleware(5*time.Second)).Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// -------------------------------------------------------------------------
	// Generation (SSE — no timeout middleware)

	s.router.Post("/generate", s.handleGenerate)

	// -------------------------------------------------------------------------
	// Static (embedded frontend)

	s.router.Handle("/*", http.FileServer(http.FS(s.staticFS)))

	return http.ListenAndServe(fmt.Sprintf(":%s", s.cfg.Port), s.router)
}
