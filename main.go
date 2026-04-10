package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/windyakin/auth-github-pat/cache"
	"github.com/windyakin/auth-github-pat/config"
	"github.com/windyakin/auth-github-pat/github"
	"github.com/windyakin/auth-github-pat/handler"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	var c cache.Cache
	switch cfg.CacheType {
	case "memory":
		c = cache.NewMemoryCache()
	case "redis":
		rc, err := cache.NewRedisCache(cfg.RedisURL)
		if err != nil {
			log.Fatalf("failed to create redis cache: %v", err)
		}
		c = rc
	}

	ghClient := github.NewClient(cfg.GitHubAPIURL)

	var membershipChecker handler.MembershipChecker
	if cfg.GitHubOrg != "" {
		appClient, err := github.NewAppClient(
			cfg.GitHubAPIURL,
			cfg.GitHubAppID,
			cfg.GitHubAppPrivateKey,
			cfg.GitHubAppInstallationID,
		)
		if err != nil {
			log.Fatalf("failed to create github app client: %v", err)
		}
		membershipChecker = appClient
	}

	authHandler := handler.NewAuthHandler(ghClient, membershipChecker, c, cfg.GitHubOrg, cfg.GitHubTeam, cfg.CacheTTL, cfg.NegativeCacheTTL)

	mux := http.NewServeMux()
	mux.Handle("GET /auth", authHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	slog.Info("server starting", "port", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler.LoggingMiddleware(mux)))
}
