package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
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
			slog.Info("auth granted", h.authLogAttrs(entry.Username, "cache", true)...)
			w.Header().Set("X-Auth-User", entry.Username)
			w.WriteHeader(http.StatusOK)
		} else {
			slog.Warn("auth denied", h.authLogAttrs(entry.Username, "cache", true)...)
			http.Error(w, "forbidden", http.StatusForbidden)
		}
		return
	}

	user, err := h.validator.GetUser(ctx, token)
	if err != nil {
		slog.Warn("token validation failed", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.org != "" {
		isMember, err := h.membershipChecker.IsOrgMember(ctx, user.Login, h.org)
		if err != nil {
			slog.Error("org membership check failed", h.authLogAttrs(user.Login, "error", err)...)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isMember {
			slog.Warn("auth denied", h.authLogAttrs(user.Login, "reason", "not_org_member")...)
			h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: false, Username: user.Login}, h.negativeCacheTTL)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	if h.team != "" {
		isMember, err := h.membershipChecker.IsTeamMember(ctx, user.Login, h.org, h.team)
		if err != nil {
			slog.Error("team membership check failed", h.authLogAttrs(user.Login, "error", err)...)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isMember {
			slog.Warn("auth denied", h.authLogAttrs(user.Login, "reason", "not_team_member")...)
			h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: false, Username: user.Login}, h.negativeCacheTTL)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	slog.Info("auth granted", h.authLogAttrs(user.Login, "cache", false)...)
	h.cache.Set(ctx, cacheKey, cache.Entry{Allowed: true, Username: user.Login}, h.cacheTTL)
	w.Header().Set("X-Auth-User", user.Login)
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) authLogAttrs(user string, extra ...any) []any {
	attrs := []any{"user", user}
	if h.org != "" {
		attrs = append(attrs, "org", h.org)
	}
	if h.team != "" {
		attrs = append(attrs, "team", h.team)
	}
	return append(attrs, extra...)
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
