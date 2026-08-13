// Package oidcclient is a provider-agnostic OAuth 2.0 Authorization-Code (+ PKCE)
// client that reads identity from the provider's UserInfo endpoint.
//
// It is deliberately generic: give it four endpoint URLs and client credentials
// and it works against ANY conforming OAuth2/OIDC provider (OpsAPI, Keycloak,
// Auth0, Google, …) — the same way you would wire a generic "Sign in with X"
// button. Because it authenticates the user by calling UserInfo over TLS (rather
// than verifying an ID-token signature), it is drop-in for providers whose ID
// tokens are signed with a shared secret and do not publish a JWKS.
package oidcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// Identity is the normalized subset of OIDC UserInfo claims beacon consumes.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Config configures the client. All fields are required except Scopes, which
// defaults (at the caller) to openid+profile+email.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	RedirectURL  string
	Scopes       []string
}

// Client performs the code exchange and UserInfo lookup.
type Client struct {
	oauth       *oauth2.Config
	userInfoURL string
	http        *http.Client
}

// New builds a Client from cfg. The client secret is sent via HTTP Basic auth
// (AuthStyleInHeader), which every conforming token endpoint accepts.
func New(cfg Config) *Client {
	return &Client{
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:   cfg.AuthorizeURL,
				TokenURL:  cfg.TokenURL,
				AuthStyle: oauth2.AuthStyleInHeader,
			},
		},
		userInfoURL: cfg.UserInfoURL,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

// GenerateVerifier returns a fresh PKCE code_verifier. Persist it against the
// request's state so the callback can complete the exchange.
func GenerateVerifier() string { return oauth2.GenerateVerifier() }

// AuthCodeURL builds the provider's authorize URL for state, binding the request
// to the PKCE S256 challenge derived from verifier.
func (c *Client) AuthCodeURL(state, verifier string) string {
	return c.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	)
}

// Exchange swaps an authorization code (with its PKCE verifier) for tokens.
func (c *Client) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.http)
	return c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
}

// UserInfo fetches and normalizes the provider's UserInfo claims using the
// access token from Exchange.
func (c *Client) UserInfo(ctx context.Context, accessToken string) (*Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo: unexpected status %d", resp.StatusCode)
	}

	var raw struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("userinfo: decode: %w", err)
	}
	return &Identity{
		Subject:       raw.Sub,
		Email:         raw.Email,
		EmailVerified: raw.EmailVerified,
		Name:          raw.Name,
	}, nil
}
