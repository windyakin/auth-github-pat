package github

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AppClient struct {
	baseURL        string
	appID          string
	privateKey     *rsa.PrivateKey
	installationID string
	httpClient     *http.Client

	mu          sync.RWMutex
	token       string
	tokenExpiry time.Time
}

func NewAppClient(baseURL, appID, privateKeyPEM, installationID string) (*AppClient, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return &AppClient{
		baseURL:        baseURL,
		appID:          appID,
		privateKey:     key,
		installationID: installationID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (ac *AppClient) getInstallationToken(ctx context.Context) (string, error) {
	ac.mu.RLock()
	if ac.token != "" && time.Now().Before(ac.tokenExpiry) {
		token := ac.token
		ac.mu.RUnlock()
		return token, nil
	}
	ac.mu.RUnlock()

	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Double-check after acquiring write lock
	if ac.token != "" && time.Now().Before(ac.tokenExpiry) {
		return ac.token, nil
	}

	jwtToken, err := ac.generateJWT()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", ac.baseURL, ac.installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	ac.token = result.Token
	// Refresh 5 minutes before expiry
	ac.tokenExpiry = result.ExpiresAt.Add(-5 * time.Minute)

	return ac.token, nil
}

func (ac *AppClient) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    ac.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(ac.privateKey)
}

func (ac *AppClient) IsOrgMember(ctx context.Context, username, org string) (bool, error) {
	token, err := ac.getInstallationToken(ctx)
	if err != nil {
		return false, fmt.Errorf("getting installation token: %w", err)
	}

	url := fmt.Sprintf("%s/orgs/%s/members/%s", ac.baseURL, org, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// 204: member, 404: not a member, 302: requester is not org member
	return resp.StatusCode == http.StatusNoContent, nil
}

func (ac *AppClient) IsTeamMember(ctx context.Context, username, org, teamSlug string) (bool, error) {
	token, err := ac.getInstallationToken(ctx)
	if err != nil {
		return false, fmt.Errorf("getting installation token: %w", err)
	}

	url := fmt.Sprintf("%s/orgs/%s/teams/%s/members/%s", ac.baseURL, org, teamSlug, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// 204: member, 404: not a member
	return resp.StatusCode == http.StatusNoContent, nil
}
