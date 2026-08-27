// Package apns is a minimal Apple Push Notification service (APNs) client. It
// signs a provider-authentication JWT (ES256) with a .p8 key and delivers a JSON
// payload to a device token over HTTP/2. It is intentionally small: one send
// path, provider-token auth (the modern key-based scheme — no per-app TLS
// certificates), and just enough error decoding to tell "retry later" from "this
// device token is dead, stop sending to it".
package apns

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// APNs has two disjoint environments; a device token minted against one is
// rejected by the other, which is the single most common "why doesn't my push
// arrive" cause — so the environment is explicit config, never guessed.
const (
	hostProduction = "https://api.push.apple.com"
	hostSandbox    = "https://api.sandbox.push.apple.com"
)

// providerTokenTTL is how long a signed provider JWT is reused before it is
// regenerated. APNs rejects tokens older than one hour, and also rejects
// too-frequent regeneration (TooManyProviderTokenUpdates), so the reuse window
// sits between: long enough to avoid churn, comfortably under the hour ceiling.
const providerTokenTTL = 50 * time.Minute

// Config configures a Client.
type Config struct {
	// PrivateKey is the parsed .p8 signing key.
	PrivateKey *ecdsa.PrivateKey
	// KeyID is the Apple Key ID (the .p8's identifier), sent as the JWT `kid`.
	KeyID string
	// TeamID is the Apple Developer Team ID (the JWT issuer).
	TeamID string
	// Topic is the app bundle id, sent as the apns-topic header.
	Topic string
	// Production selects the APNs host. False targets the sandbox, which is what
	// a development build's device tokens are valid against.
	Production bool
	// HTTPClient is optional; a sane HTTP/2 client is used when nil.
	HTTPClient *http.Client
}

// Client sends pushes to APNs. Safe for concurrent use.
type Client struct {
	cfg    Config
	host   string
	client *http.Client

	mu      sync.Mutex
	token   string
	tokenAt time.Time
	now     func() time.Time
}

// New builds a Client from an already-parsed key.
func New(cfg Config) (*Client, error) {
	if cfg.PrivateKey == nil {
		return nil, errors.New("apns: private key is required")
	}
	if cfg.KeyID == "" || cfg.TeamID == "" || cfg.Topic == "" {
		return nil, errors.New("apns: key id, team id and topic are required")
	}
	host := hostSandbox
	if cfg.Production {
		host = hostProduction
	}
	hc := cfg.HTTPClient
	if hc == nil {
		// Default transport negotiates HTTP/2 over TLS via ALPN, which APNs requires.
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{cfg: cfg, host: host, client: hc, now: time.Now}, nil
}

// ParseP8Key parses a PEM-encoded PKCS#8 EC private key (the contents of an Apple
// .p8 file) into an ECDSA key.
func ParseP8Key(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("apns: no PEM block found in signing key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: parse pkcs8 key: %w", err)
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apns: signing key is %T, want ECDSA", key)
	}
	return ec, nil
}

// Result is the outcome of one push.
type Result struct {
	// StatusCode is the APNs HTTP status (200 = accepted by APNs).
	StatusCode int
	// Reason is APNs' machine-readable failure reason (e.g. "BadDeviceToken",
	// "Unregistered"), empty on success.
	Reason string
	// APNsID correlates the send with Apple's logs.
	APNsID string
}

// OK reports a successful hand-off to APNs.
func (r Result) OK() bool { return r.StatusCode == http.StatusOK }

// TokenIsDead reports that the device token will never work again and should be
// pruned. APNs signals this as 410 Gone (Unregistered) or 400 with a token-shape
// reason. Anything else (5xx, quota) is transient — keep the token.
func (r Result) TokenIsDead() bool {
	if r.StatusCode == http.StatusGone {
		return true
	}
	switch r.Reason {
	case "Unregistered", "BadDeviceToken", "DeviceTokenNotForTopic":
		return true
	default:
		return false
	}
}

// Push delivers a raw JSON payload to one device token.
func (c *Client) Push(ctx context.Context, deviceToken string, payload []byte) (Result, error) {
	jwtTok, err := c.providerToken()
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/3/device/"+deviceToken, bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("apns: build request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+jwtTok)
	req.Header.Set("apns-topic", c.cfg.Topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("apns: request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	res := Result{StatusCode: resp.StatusCode, APNsID: resp.Header.Get("apns-id")}
	if resp.StatusCode != http.StatusOK {
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		res.Reason = body.Reason
	}
	return res, nil
}

// providerToken returns a cached provider JWT, regenerating it once it ages past
// providerTokenTTL. The lock is held only for the cheap cache check and the
// occasional re-sign.
func (c *Client) providerToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Sub(c.tokenAt) < providerTokenTTL {
		return c.token, nil
	}
	now := c.now()
	claims := jwt.MapClaims{"iss": c.cfg.TeamID, "iat": now.Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = c.cfg.KeyID
	signed, err := tok.SignedString(c.cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("apns: sign provider token: %w", err)
	}
	c.token, c.tokenAt = signed, now
	return signed, nil
}
