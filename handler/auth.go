package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/windyakin/auth-github-pat/cache"
	"github.com/windyakin/auth-github-pat/github"
)

type TokenValidator interface {
	GetUser(ctx context.Context, token string) (*github.User, error)
}

type MembershipChecker interface {
	IsOrgMember(ctx context.Context, username, org string) (bool, error)
	IsTeamMember(ctx context.Context, username, org, teamSlug string) (bool, error)
}

type AuthHandler struct {
	validator         TokenValidator
	membershipChecker MembershipChecker
	cache             cache.Cache
	org               string
	team              string
	cacheTTL          time.Duration
	negativeCacheTTL  time.Duration
}

func NewAuthHandler(validator TokenValidator, membershipChecker MembershipChecker, c cache.Cache, org, team string, cacheTTL, negativeCacheTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		validator:         validator,
		membershipChecker: membershipChecker,
		cache:             c,
		org:               org,
		team:              team,
		cacheTTL:          cacheTTL,
		negativeCacheTTL:  negativeCacheTTL,
	}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	cacheKey := h.buildCacheKey(token)
	ctx := r.Context()

	if entry, ok := h.cache.Get(ctx, cacheKey); ok {
		if entry.Allowed {
			w.Header().Set("X-Auth-User", entry.Username)
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "forbidden", http.StatusForbidden)
		}
		return
	}

	user, err := h.validator.GetUser(ctx, token)
	if err != nil {
		log.Printf("token validation failed: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.org != "" {
		isMember, err := h.membershipChecker.IsOrgMember(ctx, user.Login, h.org)
		if err != nil {
			log.Printf("org membership check failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isMember {
			h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: false, Username: user.Login}, h.negativeCacheTTL)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	if h.team != "" {
		isMember, err := h.membershipChecker.IsTeamMember(ctx, user.Login, h.org, h.team)
		if err != nil {
			log.Printf("team membership check failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isMember {
			h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: false, Username: user.Login}, h.negativeCacheTTL)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: true, Username: user.Login}, h.cacheTTL)
	w.Header().Set("X-Auth-User", user.Login)
	w.WriteHeader(http.StatusOK)
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func (h *AuthHandler) buildCacheKey(token string) string {
	raw := token + "\x00" + h.org + "\x00" + h.team
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("sha256:%x", sum)
}
