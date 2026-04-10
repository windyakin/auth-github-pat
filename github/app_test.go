package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

func generateTestPrivateKey(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemBytes), key
}

func TestAppClient_IsOrgMember(t *testing.T) {
	pemStr, _ := generateTestPrivateKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/123/access_tokens":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"token":"install-token","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/orgs/my-org/members/testuser":
			if r.Header.Get("Authorization") != "Bearer install-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "/orgs/my-org/members/outsider":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewAppClient(srv.URL, "1", pemStr, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isMember, err := client.IsOrgMember(context.Background(), "testuser", "my-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isMember {
		t.Fatal("expected testuser to be a member")
	}

	isMember, err = client.IsOrgMember(context.Background(), "outsider", "my-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isMember {
		t.Fatal("expected outsider to not be a member")
	}
}

func TestAppClient_IsTeamMember(t *testing.T) {
	pemStr, _ := generateTestPrivateKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/123/access_tokens":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"token":"install-token","expires_at":"2099-01-01T00:00:00Z"}`))
		case "/orgs/my-org/teams/my-team/members/testuser":
			w.WriteHeader(http.StatusNoContent)
		case "/orgs/my-org/teams/my-team/members/outsider":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewAppClient(srv.URL, "1", pemStr, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isMember, err := client.IsTeamMember(context.Background(), "testuser", "my-org", "my-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isMember {
		t.Fatal("expected testuser to be a team member")
	}

	isMember, err = client.IsTeamMember(context.Background(), "outsider", "my-org", "my-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isMember {
		t.Fatal("expected outsider to not be a team member")
	}
}
