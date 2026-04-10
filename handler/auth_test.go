package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/windyakin/auth-github-pat/cache"
	"github.com/windyakin/auth-github-pat/github"
)

type mockValidator struct {
	user *github.User
	err  error
}

func (m *mockValidator) GetUser(_ context.Context, _ string) (*github.User, error) {
	return m.user, m.err
}

type mockMembershipChecker struct {
	orgMember  bool
	orgErr     error
	teamMember bool
	teamErr    error
}

func (m *mockMembershipChecker) IsOrgMember(_ context.Context, _, _ string) (bool, error) {
	return m.orgMember, m.orgErr
}

func (m *mockMembershipChecker) IsTeamMember(_ context.Context, _, _, _ string) (bool, error) {
	return m.teamMember, m.teamErr
}

var defaultTTL = 5 * time.Minute

func TestAuthHandler_MissingHeader(t *testing.T) {
	h := NewAuthHandler(&mockValidator{}, nil, cache.NewMemoryCache(), "", "", defaultTTL, 0)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHandler_InvalidToken(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{err: fmt.Errorf("invalid token")},
		nil,
		cache.NewMemoryCache(),
		"", "",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHandler_ValidToken_NoOrgCheck(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		nil,
		cache.NewMemoryCache(),
		"", "",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Auth-User") != "testuser" {
		t.Fatalf("expected X-Auth-User=testuser, got %s", rec.Header().Get("X-Auth-User"))
	}
}

func TestAuthHandler_ValidToken_OrgMember(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: true},
		cache.NewMemoryCache(),
		"my-org", "",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthHandler_ValidToken_NotOrgMember(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: false},
		cache.NewMemoryCache(),
		"my-org", "",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAuthHandler_ValidToken_TeamMember(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: true, teamMember: true},
		cache.NewMemoryCache(),
		"my-org", "my-team",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthHandler_ValidToken_NotTeamMember(t *testing.T) {
	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: true, teamMember: false},
		cache.NewMemoryCache(),
		"my-org", "my-team",
		defaultTTL, 0,
	)
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAuthHandler_CacheHit(t *testing.T) {
	mc := cache.NewMemoryCache()
	validator := &mockValidator{user: &github.User{Login: "testuser"}}

	h := NewAuthHandler(validator, nil, mc, "", "", defaultTTL, 0)

	// First request - populates cache
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer cached-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Replace validator with one that tracks calls
	countingValidator := &countingMockValidator{user: &github.User{Login: "testuser"}}
	h.validator = countingValidator

	// Second request - should hit cache
	req = httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer cached-token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if countingValidator.callCount != 0 {
		t.Fatalf("expected 0 calls to validator on cache hit, got %d", countingValidator.callCount)
	}
}

func TestAuthHandler_NegativeCacheDisabled(t *testing.T) {
	mc := cache.NewMemoryCache()

	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: false},
		mc,
		"my-org", "",
		defaultTTL, 0, // negativeCacheTTL = 0 → disabled
	)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer denied-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	// Replace membership checker to allow
	h.membershipChecker = &mockMembershipChecker{orgMember: true}

	// Second request - should NOT hit cache (negative cache disabled)
	req = httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer denied-token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after membership change, got %d", rec.Code)
	}
}

func TestAuthHandler_NegativeCacheEnabled(t *testing.T) {
	mc := cache.NewMemoryCache()

	h := NewAuthHandler(
		&mockValidator{user: &github.User{Login: "testuser"}},
		&mockMembershipChecker{orgMember: false},
		mc,
		"my-org", "",
		defaultTTL, 1*time.Minute, // negativeCacheTTL = 1m → enabled
	)

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer denied-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	// Replace membership checker to allow
	h.membershipChecker = &mockMembershipChecker{orgMember: true}

	// Second request - should hit negative cache
	req = httptest.NewRequest(http.MethodGet, "/auth", nil)
	req.Header.Set("Authorization", "Bearer denied-token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from negative cache, got %d", rec.Code)
	}
}

type countingMockValidator struct {
	user      *github.User
	callCount int
}

func (m *countingMockValidator) GetUser(_ context.Context, _ string) (*github.User, error) {
	m.callCount++
	return m.user, nil
}
