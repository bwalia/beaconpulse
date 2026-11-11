// Package fcm is a minimal Firebase Cloud Messaging (FCM) HTTP v1 client. It
// authenticates as a service account (an OAuth2 JWT-bearer grant, cached until
// it expires) and posts one message per call — the Android twin of package apns,
// with just enough error decoding to tell "retry later" from "this registration
// token is dead, stop sending to it".
package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	endpoint = "https://fcm.googleapis.com"
	tokenURL = "https://oauth2.googleapis.com/token"
	scope    = "https://www.googleapis.com/auth/firebase.messaging"
)

// Config is the service account FCM sends as (from its key JSON).
type Config struct {
	ProjectID   string
	ClientEmail string
	PrivateKey  string // PEM
	// HTTPClient is optional; a client with a 10s timeout is used when nil.
	HTTPClient *http.Client
}

// Client sends messages to FCM. Safe for concurrent use.
type Client struct {
	url    string
	tokens oauth2.TokenSource
	client *http.Client
}

// New builds a Client. The access token is minted on first send and reused
// until shortly before it expires.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	jc := &jwt.Config{
		Email:      cfg.ClientEmail,
		PrivateKey: []byte(cfg.PrivateKey),
		Scopes:     []string{scope},
		TokenURL:   tokenURL,
	}
	// The token exchange uses hc too, so it gets the same timeout.
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, hc)
	return &Client{
		url:    endpoint + "/v1/projects/" + url.PathEscape(cfg.ProjectID) + "/messages:send",
		tokens: jc.TokenSource(ctx), // a ReuseTokenSource: cached and refreshed
		client: hc,
	}
}

// Result is the outcome of one send.
type Result struct {
	// StatusCode is the HTTP status (200 = accepted by FCM).
	StatusCode int
	// Code is FCM's error code (e.g. "UNREGISTERED", "INVALID_ARGUMENT"), empty on success.
	Code string
	// Message is Google's human-readable error message.
	Message string
	// Field is the request field a 400 blames, e.g. "message.token".
	Field string
}

// OK reports a successful hand-off to FCM.
func (r Result) OK() bool { return r.StatusCode == http.StatusOK }

// TokenIsDead reports that the registration token will never work again and
// should be pruned: UNREGISTERED (404), or a 400 INVALID_ARGUMENT about the
// token itself. Auth, quota and server errors (401/403/429/5xx) keep the token.
func (r Result) TokenIsDead() bool {
	if r.Code == "UNREGISTERED" {
		return true
	}
	return r.StatusCode == http.StatusBadRequest && r.Code == "INVALID_ARGUMENT" &&
		(r.Field == "message.token" || strings.Contains(strings.ToLower(r.Message), "registration token"))
}

// Send posts a messages:send request body ({"message": {...}}).
func (c *Client) Send(ctx context.Context, body []byte) (Result, error) {
	tok, err := c.tokens.Token()
	if err != nil {
		return Result{}, fmt.Errorf("fcm: access token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("fcm: build request: %w", err)
	}
	tok.SetAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fcm: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	res := Result{StatusCode: resp.StatusCode}
	if resp.StatusCode != http.StatusOK {
		// google.rpc.Status: the FCM error code and any blamed field live in details.
		var e struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
				Details []struct {
					ErrorCode       string `json:"errorCode"`
					FieldViolations []struct {
						Field string `json:"field"`
					} `json:"fieldViolations"`
				} `json:"details"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		res.Code, res.Message = e.Error.Status, e.Error.Message
		for _, d := range e.Error.Details {
			if d.ErrorCode != "" {
				res.Code = d.ErrorCode
			}
			for _, fv := range d.FieldViolations {
				res.Field = fv.Field
			}
		}
	}
	return res, nil
}
