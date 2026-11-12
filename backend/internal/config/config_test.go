package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

const validKey = "00000000000000000000000000000000000000000000000000000000000000ff" // 32 bytes hex

func setValidBase(t *testing.T) {
	t.Helper()
	t.Setenv("BEACON_DB_DSN", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("BEACON_JWT_ACCESS_SECRET", "access-secret-at-least-32-bytes-long-xx")
	t.Setenv("BEACON_JWT_REFRESH_SECRET", "refresh-secret-at-least-32-bytes-long-x")
	t.Setenv("BEACON_ENCRYPTION_KEY", validKey)
}

func TestLoadValid(t *testing.T) {
	setValidBase(t)
	t.Setenv("BEACON_ENV", "staging")
	t.Setenv("BEACON_HTTP_ADDR", ":9999")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != EnvStaging {
		t.Errorf("env = %q", cfg.Env)
	}
	if cfg.HTTP.Addr != ":9999" {
		t.Errorf("addr = %q", cfg.HTTP.Addr)
	}
	if len(cfg.Crypto.EncryptionKey) != 32 {
		t.Errorf("encryption key length = %d, want 32", len(cfg.Crypto.EncryptionKey))
	}
}

func TestLoadAIEnabledRequiresURLAndModel(t *testing.T) {
	setValidBase(t)
	t.Setenv("BEACON_AI_ENABLED", "true")
	// BEACON_AI_BASE_URL and BEACON_AI_MODEL intentionally unset.
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BEACON_AI_BASE_URL") || !strings.Contains(err.Error(), "BEACON_AI_MODEL") {
		t.Fatalf("expected AI url+model errors, got %v", err)
	}
}

func TestLoadAIEnabledValid(t *testing.T) {
	setValidBase(t)
	t.Setenv("BEACON_AI_ENABLED", "true")
	t.Setenv("BEACON_AI_BASE_URL", "https://ollama.example.com/")
	t.Setenv("BEACON_AI_MODEL", "llama3.1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AI.Enabled {
		t.Error("AI should be enabled")
	}
	if cfg.AI.BaseURL != "https://ollama.example.com" { // trailing slash trimmed
		t.Errorf("base url = %q", cfg.AI.BaseURL)
	}
}

func TestLoadMissingDSN(t *testing.T) {
	t.Setenv("BEACON_JWT_ACCESS_SECRET", "access-secret-at-least-32-bytes-long-xx")
	t.Setenv("BEACON_JWT_REFRESH_SECRET", "refresh-secret-at-least-32-bytes-long-x")
	t.Setenv("BEACON_ENCRYPTION_KEY", validKey)
	// BEACON_DB_DSN intentionally unset.
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BEACON_DB_DSN") {
		t.Fatalf("expected DSN error, got %v", err)
	}
}

func TestLoadShortSecret(t *testing.T) {
	setValidBase(t)
	t.Setenv("BEACON_JWT_ACCESS_SECRET", "too-short")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ACCESS_SECRET") {
		t.Fatalf("expected access secret error, got %v", err)
	}
}

func TestProductionRejectsDefaultSecrets(t *testing.T) {
	setValidBase(t)
	t.Setenv("BEACON_ENV", "production")
	t.Setenv("BEACON_JWT_ACCESS_SECRET", "dev-access-secret-change-me-000000000000")
	t.Setenv("BEACON_JWT_REFRESH_SECRET", "dev-refresh-secret-change-me-00000000000")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "change-me") {
		t.Fatalf("expected refusal of default secrets in production, got %v", err)
	}
}

// fcmKeyJSON returns a Firebase service-account key JSON with a fresh RSA key.
func fcmKeyJSON(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	b, _ := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "beacon-demo",
		"client_email": "push@beacon-demo.iam.gserviceaccount.com",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	})
	return string(b)
}

func TestFCMCredentials(t *testing.T) {
	setValidBase(t)
	cfg, err := Load()
	if err != nil || cfg.Push.FCM.Enabled() {
		t.Fatalf("unset: want Android push off and no error, got enabled=%v err=%v", cfg.Push.FCM.Enabled(), err)
	}

	key := fcmKeyJSON(t)
	t.Setenv("BEACON_FCM_CREDENTIALS", key)
	cfg, err = Load()
	if err != nil || !cfg.Push.FCM.Enabled() || cfg.Push.FCM.ProjectID != "beacon-demo" {
		t.Fatalf("valid key: enabled=%v project=%q err=%v", cfg.Push.FCM.Enabled(), cfg.Push.FCM.ProjectID, err)
	}

	// A single-line secret store can leave the key's newlines as literal "\n".
	t.Setenv("BEACON_FCM_CREDENTIALS", strings.ReplaceAll(key, `\n`, `\\n`))
	if cfg, err = Load(); err != nil || !cfg.Push.FCM.Enabled() {
		t.Fatalf("escaped newlines: %v", err)
	}

	for name, raw := range map[string]string{
		"not json":       "/path/to/key.json",
		"missing fields": `{"project_id":"p"}`,
		"bad key":        `{"project_id":"p","client_email":"e","private_key":"nope"}`,
	} {
		t.Setenv("BEACON_FCM_CREDENTIALS", raw)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "BEACON_FCM_CREDENTIALS") {
			t.Errorf("%s: want a BEACON_FCM_CREDENTIALS error, got %v", name, err)
		}
	}
}
