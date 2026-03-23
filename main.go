package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/noahschumacher/llm-txt/pkg"
	"github.com/noahschumacher/llm-txt/server"
)

//go:embed static
var staticFiles embed.FS

const (
	appEnvLocal = "local"
	appEnvDev   = "dev"
	appEnvProd  = "prod"
)

func main() {
	// -------------------------------------------------------------------------
	// Load Environment Variables

	envFile := os.Getenv("LLM_TXT_ENV_FILE")
	if envFile != "" {
		log.Printf("loading environment variables from %s\n", envFile)
		if err := godotenv.Load(envFile); err != nil {
			log.Fatalf("error loading env file: %v", err)
		}
	}

	var (
		AppEnv  = pkg.LoadStringEnv("APP_ENV", true)
		AppPort = pkg.LoadStringEnv("APP_PORT", true)
	)

	var (
		LLMProvider    = pkg.LoadStringEnv("LLM_PROVIDER", false)
		LLMAPIKey      = pkg.LoadStringEnv("LLM_API_KEY", false)
		LLMModel       = pkg.LoadStringEnv("LLM_MODEL", false)
		LLMConcurrency = pkg.LoadIntEnv("LLM_CONCURRENCY", false)
	)

	var (
		CrawlMaxPages = pkg.LoadIntEnv("CRAWL_MAX_PAGES", false)
		CrawlMaxDepth = pkg.LoadIntEnv("CRAWL_MAX_DEPTH", false)
		CrawlDelayMS  = pkg.LoadIntEnv("CRAWL_DELAY_MS", false)
	)

	// -------------------------------------------------------------------------
	// Initialize Logger

	var logConfig zap.Config
	if AppEnv == appEnvLocal {
		logConfig = zap.NewDevelopmentConfig()
		logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		logConfig = zap.NewProductionConfig()
		if AppEnv == appEnvDev {
			logConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		}
	}

	logger, err := logConfig.Build()
	if err != nil {
		log.Fatalf("error creating logger: %v", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	logger.Info("environment variables loaded",
		zap.String("app_env", AppEnv),
		zap.String("app_port", AppPort),
		zap.String("llm_provider", LLMProvider),
		zap.String("llm_api_key", mask(LLMAPIKey)),
		zap.String("llm_model", LLMModel),
		zap.Int("llm_concurrency", LLMConcurrency),
		zap.Int("crawl_max_pages", CrawlMaxPages),
		zap.Int("crawl_max_depth", CrawlMaxDepth),
		zap.Int("crawl_delay_ms", CrawlDelayMS),
	)

	// -------------------------------------------------------------------------
	// Start HTTP Server

	// Serve static files rooted at the static/ subdirectory so that "/" maps
	// to static/index.html rather than the embed root.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("error creating static fs: %v", err)
	}

	srv := server.New(logger, server.Config{
		Port:   AppPort,
		AppEnv: AppEnv,
	}, staticFS)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.ListenAndServe()
	}()

	// -------------------------------------------------------------------------
	// Shutdown Handling

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Fatal("server error", zap.Error(err))
	case <-shutdown:
		logger.Info("shutting down")
	}
}

func mask(s string) string {
	if s == "" {
		return ""
	}
	return "********"
}
