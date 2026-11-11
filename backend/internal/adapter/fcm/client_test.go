package fcm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

// testClient points a Client at srv with a static access token, so no Google
// endpoint is touched.
func testClient(srv *httptest.Server) *Client {
	return &Client{
		url:    srv.URL + "/v1/projects/demo/messages:send",
		tokens: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "access-123", TokenType: "Bearer"}),
		client: srv.Client(),
	}
}

func TestNewTargetsProjectSendURL(t *testing.T) {
	c := New(Config{ProjectID: "my-proj", ClientEmail: "sa@my-proj.iam.gserviceaccount.com", PrivateKey: "pem"})
	if want := "https://fcm.googleapis.com/v1/projects/my-proj/messages:send"; c.url != want {
		t.Fatalf("url=%q want %q", c.url, want)
	}
}

func TestSendPostsBodyWithBearer(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"name":"projects/demo/messages/1"}`))
	}))
	defer srv.Close()

	res, err := testClient(srv).Send(context.Background(), []byte(`{"message":{"token":"t"}}`))
	if err != nil || !res.OK() {
		t.Fatalf("Send = %+v, %v", res, err)
	}
	if gotAuth != "Bearer access-123" {
		t.Errorf("Authorization=%q", gotAuth)
	}
	if gotPath != "/v1/projects/demo/messages:send" {
		t.Errorf("path=%q", gotPath)
	}
	if gotBody != `{"message":{"token":"t"}}` {
		t.Errorf("body=%q", gotBody)
	}
}

// TestSendDecodesErrorsAndPruneRules runs real FCM v1 error bodies through Send:
// only an unregistered token, or a 400 that blames the token, is dead.
func TestSendDecodesErrorsAndPruneRules(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		dead   bool
	}{
		{"unregistered", 404, `{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND",
			"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`, true},
		{"invalid token field", 400, `{"error":{"code":400,"message":"The registration token is not a valid FCM registration token","status":"INVALID_ARGUMENT",
			"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"},
			{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.token","description":"Invalid registration token"}]}]}}`, true},
		{"invalid token message only", 400, `{"error":{"code":400,"message":"The registration token is not a valid FCM registration token","status":"INVALID_ARGUMENT"}}`, true},
		{"invalid payload", 400, `{"error":{"code":400,"message":"Invalid value at 'message.data[0].value'","status":"INVALID_ARGUMENT",
			"details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.data[0].value"}]}]}}`, false},
		{"bare 404", 404, `not json`, false},
		{"unauthenticated", 401, `{"error":{"code":401,"status":"UNAUTHENTICATED"}}`, false},
		{"sender mismatch", 403, `{"error":{"code":403,"status":"PERMISSION_DENIED","details":[{"errorCode":"SENDER_ID_MISMATCH"}]}}`, false},
		{"quota", 429, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"errorCode":"QUOTA_EXCEEDED"}]}}`, false},
		{"unavailable", 503, `{"error":{"code":503,"status":"UNAVAILABLE","details":[{"errorCode":"UNAVAILABLE"}]}}`, false},
		{"internal", 500, `{"error":{"code":500,"status":"INTERNAL","details":[{"errorCode":"INTERNAL"}]}}`, false},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		res, err := testClient(srv).Send(context.Background(), []byte(`{}`))
		srv.Close()
		if err != nil {
			t.Fatalf("%s: Send: %v", c.name, err)
		}
		if res.OK() || res.StatusCode != c.status {
			t.Errorf("%s: result %+v", c.name, res)
		}
		if got := res.TokenIsDead(); got != c.dead {
			t.Errorf("%s: TokenIsDead()=%v want %v (%+v)", c.name, got, c.dead, res)
		}
	}
}
