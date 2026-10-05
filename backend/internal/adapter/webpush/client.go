// Package webpush is a minimal Web Push sender for browser (PWA) notifications.
// It encrypts a payload for one subscription (RFC 8291, aes128gcm content coding
// from RFC 8188), signs a VAPID token (RFC 8292) and POSTs it to the
// subscription's push service (RFC 8030). Like the apns adapter it is deliberately
// small: one send path, standard library crypto, and just enough result decoding
// to tell "delivered" from "this subscription is gone, stop sending to it".
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// recordSize is the aes128gcm record size advertised in the header. A push
// message is a single record, and push services cap the whole body at 4096 bytes,
// so the plaintext limit is that minus the header, delimiter and GCM tag.
const (
	recordSize      = 4096
	headerLen       = 16 + 4 + 1 + 65
	MaxPayloadBytes = recordSize - headerLen - 1 - 16
)

// Subscription is what a browser's PushManager.subscribe() returns: the push
// service endpoint plus the browser's ECDH public key and auth secret (base64url).
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Result is the push service's answer to one send.
type Result struct {
	StatusCode int
	Body       string
}

// OK reports the push service accepted the message (201 Created, usually).
func (r Result) OK() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// Gone reports the subscription will never work again (the browser unsubscribed
// or the push service expired it) and should be deleted.
func (r Result) Gone() bool {
	return r.StatusCode == http.StatusNotFound || r.StatusCode == http.StatusGone
}

// Doer sends an HTTP request — an *http.Client, or the SSRF-guarded safehttp client.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client sends web push messages. Safe for concurrent use.
type Client struct {
	key     *ecdh.PrivateKey
	signer  *ecdsa.PrivateKey
	public  string // base64url uncompressed public key: the browser's applicationServerKey
	subject string
	http    Doer
	now     func() time.Time
	// Overridable so the encryption can be checked against RFC 8291's fixed vector.
	ephemeral func() (*ecdh.PrivateKey, error)
	salt      func() ([]byte, error)
}

// New builds a Client from the VAPID private key (a raw 32-byte P-256 scalar,
// base64url or standard base64) and a contact subject (mailto: or https:).
func New(privateKey, subject string, httpClient Doer) (*Client, error) {
	raw, err := DecodeKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key: %w", err)
	}
	key, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key: %w", err)
	}
	// The JWT library signs with crypto/ecdsa; PKCS#8 is the stdlib bridge between
	// the two representations of the same P-256 key.
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("webpush: private key: %w", err)
	}
	if !strings.HasPrefix(subject, "mailto:") && !strings.HasPrefix(subject, "https://") {
		return nil, errors.New("webpush: subject must be a mailto: or https: URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		key:       key,
		signer:    parsed.(*ecdsa.PrivateKey),
		public:    base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		subject:   subject,
		http:      httpClient,
		now:       time.Now,
		ephemeral: func() (*ecdh.PrivateKey, error) { return ecdh.P256().GenerateKey(rand.Reader) },
		salt: func() ([]byte, error) {
			b := make([]byte, 16)
			_, err := rand.Read(b)
			return b, err
		},
	}, nil
}

// GenerateKey returns a new VAPID private key, in the form New accepts.
func GenerateKey() (string, error) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(k.Bytes()), nil
}

// PublicKey is the VAPID public key browsers pass to PushManager.subscribe().
func (c *Client) PublicKey() string { return c.public }

// Send encrypts payload for sub and delivers it. ttl bounds how long the push
// service holds it for an offline device; urgency is "high" or "normal".
func (c *Client) Send(ctx context.Context, sub Subscription, payload []byte, ttl time.Duration, urgency string) (Result, error) {
	if len(payload) > MaxPayloadBytes {
		return Result{}, fmt.Errorf("webpush: payload is %d bytes, max %d", len(payload), MaxPayloadBytes)
	}
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return Result{}, errors.New("webpush: endpoint must be an https URL")
	}
	uaPub, err := DecodeKey(sub.P256dh)
	if err != nil {
		return Result{}, fmt.Errorf("webpush: p256dh: %w", err)
	}
	auth, err := DecodeKey(sub.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("webpush: auth: %w", err)
	}
	eph, err := c.ephemeral()
	if err != nil {
		return Result{}, err
	}
	salt, err := c.salt()
	if err != nil {
		return Result{}, err
	}
	body, err := encrypt(payload, uaPub, auth, eph, salt)
	if err != nil {
		return Result{}, err
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"aud": u.Scheme + "://" + u.Host,
		"exp": c.now().Add(12 * time.Hour).Unix(), // RFC 8292 caps it at 24h
		"sub": c.subject,
	}).SignedString(c.signer)
	if err != nil {
		return Result{}, fmt.Errorf("webpush: sign vapid token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("webpush: build request: %w", err)
	}
	req.Header.Set("Authorization", "vapid t="+token+", k="+c.public)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Urgency", urgency)

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("webpush: request failed: %w", err)
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return Result{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(msg))}, nil
}

// encrypt produces the aes128gcm body for one message (RFC 8291 §3.4): an ECDH
// secret between a fresh server key and the browser's key, mixed with the
// browser's auth secret, derives the content key and nonce for a single record.
func encrypt(plaintext, uaPub, auth []byte, eph *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	ua, err := ecdh.P256().NewPublicKey(uaPub)
	if err != nil {
		return nil, fmt.Errorf("webpush: p256dh: %w", err)
	}
	if len(auth) != 16 || len(salt) != 16 {
		return nil, errors.New("webpush: auth secret and salt must be 16 bytes")
	}
	secret, err := eph.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPub := eph.PublicKey().Bytes()

	keyInfo := append(append([]byte("WebPush: info\x00"), uaPub...), asPub...)
	ikm, err := hkdf.Key(sha256.New, secret, auth, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, headerLen+len(plaintext)+1+gcm.Overhead())
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, recordSize)
	out = append(out, byte(len(asPub)))
	out = append(out, asPub...)
	// 0x02 marks the last (only) record; no padding.
	return gcm.Seal(out, nonce, append(plaintext, 0x02), nil), nil
}

// DecodeKey accepts base64url (what browsers emit) or standard base64, with or
// without padding.
func DecodeKey(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return base64.RawURLEncoding.DecodeString(s)
}
