package webpush

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestEncryptMatchesRFC8291 reproduces the worked example in RFC 8291 §5 byte for
// byte: fixed server key, browser key, auth secret and salt in, the exact message
// body out. Any slip in the key derivation or record framing fails it.
func TestEncryptMatchesRFC8291(t *testing.T) {
	b := func(s string) []byte {
		v, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	as, err := ecdh.P256().NewPrivateKey(b("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	uaPub := b("BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4")
	auth := b("BTBZMqHH6r4Tts7J_aSIgg")
	salt := b("DGv6ra1nlYgDCS1FRnbzlw")

	got, err := encrypt([]byte("When I grow up, I want to be a watermelon"), uaPub, auth, as, salt)
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if enc := base64.RawURLEncoding.EncodeToString(got); enc != want {
		t.Fatalf("body mismatch\n got %s\nwant %s", enc, want)
	}
}

// TestSendSignsAndClassifies: the request carries a VAPID token for the endpoint's
// origin and the aes128gcm headers, and a 410 reads as a dead subscription.
func TestSendSignsAndClassifies(t *testing.T) {
	var gotAuth, gotEnc, gotTTL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotEnc, gotTTL = r.Header.Get("Authorization"), r.Header.Get("Content-Encoding"), r.Header.Get("TTL")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	c, err := New("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw", "https://example.com", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicKey() != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Errorf("derived public key = %s", c.PublicKey())
	}
	res, err := c.Send(context.Background(), Subscription{
		Endpoint: srv.URL + "/push/abc",
		P256dh:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
	}, []byte(`{"title":"hi"}`), time.Hour, "high")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Gone() {
		t.Errorf("410 not classified as gone: %+v", res)
	}
	if !strings.HasPrefix(gotAuth, "vapid t=") || !strings.HasSuffix(gotAuth, ", k="+c.PublicKey()) {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotEnc != "aes128gcm" || gotTTL != "3600" {
		t.Errorf("Content-Encoding=%q TTL=%q", gotEnc, gotTTL)
	}
}
