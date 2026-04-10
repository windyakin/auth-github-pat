package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port                    string
	GitHubAPIURL            string
	GitHubOrg               string
	GitHubTeam              string
	GitHubAppID             string
	GitHubAppPrivateKey     string
	GitHubAppInstallationID string
	CacheType               string
	CacheTTL                time.Duration
	NegativeCacheTTL        time.Duration
	RedisURL                string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		GitHubAPIURL: getEnv("GITHUB_API_URL", "https://api.github.com"),
		GitHubOrg:    os.Getenv("GITHUB_ORG"),
		GitHubTeam:   os.Getenv("GITHUB_TEAM"),
		GitHubAppID:             os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey:     os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitHubAppInstallationID: os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		CacheType:               getEnv("CACHE_TYPE", "memory"),
		RedisURL:                os.Getenv("REDIS_URL"),
	}

	ttlStr := getEnv("CACHE_TTL", "15m")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_TTL %q: %w", ttlStr, err)
	}
	cfg.CacheTTL = ttl

	negativeTTLStr := getEnv("NEGATIVE_CACHE_TTL", "0")
	negativeTTL, err := time.ParseDuration(negativeTTLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid NEGATIVE_CACHE_TTL %q: %w", negativeTTLStr, err)
	}
	cfg.NegativeCacheTTL = negativeTTL

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.GitHubTeam != "" && c.GitHubOrg == "" {
		return fmt.Errorf("GITHUB_ORG is required when GITHUB_TEAM is set")
	}

	if c.GitHubOrg != "" {
		if c.GitHubAppID == "" || c.GitHubAppPrivateKey == "" || c.GitHubAppInstallationID == "" {
			return fmt.Errorf("GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, and GITHUB_APP_INSTALLATION_ID are required when GITHUB_ORG is set")
		}
	}

	if c.CacheType == "redis" && c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required when CACHE_TYPE is redis")
	}

	if c.CacheType != "memory" && c.CacheType != "redis" {
		return fmt.Errorf("CACHE_TYPE must be \"memory\" or \"redis\", got %q", c.CacheType)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
