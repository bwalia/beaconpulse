package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"beacon/internal/domain/billing"
	"beacon/internal/domain/plan"
)

const testSecret = "whsec_test_secret_for_signing_0000000000"

// sign builds the Stripe-Signature header exactly as Stripe does: HMAC-SHA256 over
// "<timestamp>.<payload>" keyed by the endpoint secret.
func sign(t *testing.T, payload string, secret string) string {
	t.Helper()
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts, payload)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

// TestParseWebhookAcceptsSDKAPIVersion is the regression test for the outage where
// every real webhook was rejected: the endpoint sent events built for the account's
// API version while the SDK was pinned to an older one, and ConstructEvent refused
// them. The payload below carries the SDK's own pinned version, which is what a
// correctly-matched endpoint sends — if this fails, the SDK and the endpoint have
// drifted apart again and live webhooks are broken.
func TestParseWebhookAcceptsSDKAPIVersion(t *testing.T) {
	orgID := "11111111-1111-1111-1111-111111111111"
	payload := fmt.Sprintf(`{
		"id": "evt_test_topup",
		"object": "event",
		"api_version": %q,
		"type": "checkout.session.completed",
		"data": {"object": {
			"id": "cs_test_1",
			"object": "checkout.session",
			"mode": "payment",
			"amount_total": 500,
			"metadata": {"kind": "topup", "org_id": %q}
		}}
	}`, stripe.APIVersion, orgID)

	c := &Client{webhookSecret: testSecret}
	ev, err := c.ParseWebhook([]byte(payload), sign(t, payload, testSecret))
	if err != nil {
		t.Fatalf("ParseWebhook rejected an event carrying the SDK's own API version (%s): %v", stripe.APIVersion, err)
	}
	if ev.Kind != billing.KindTopUp {
		t.Errorf("kind = %v, want KindTopUp", ev.Kind)
	}
	if ev.AmountCents != 500 {
		t.Errorf("amount = %d, want 500", ev.AmountCents)
	}
	if ev.OrgID.String() != orgID {
		t.Errorf("org = %s, want %s", ev.OrgID, orgID)
	}
}

// TestParseWebhookSubscriptionPeriodEndFromItem pins the other half of the upgrade:
// Stripe moved current_period_end off the subscription and onto its items, so a
// period read from the old location would silently come back as the zero time.
func TestParseWebhookSubscriptionPeriodEndFromItem(t *testing.T) {
	orgID := "22222222-2222-2222-2222-222222222222"
	periodEndUnix := int64(1893456000) // 2030-01-01T00:00:00Z
	payload := fmt.Sprintf(`{
		"id": "evt_test_sub",
		"object": "event",
		"api_version": %q,
		"type": "customer.subscription.updated",
		"data": {"object": {
			"id": "sub_test_1",
			"object": "subscription",
			"status": "active",
			"metadata": {"org_id": %q},
			"items": {"object": "list", "data": [
				{"id": "si_1", "object": "subscription_item", "current_period_end": %d,
				 "price": {"id": "price_starter_test", "object": "price"}}
			]}
		}}
	}`, stripe.APIVersion, orgID, periodEndUnix)

	c := &Client{webhookSecret: testSecret, priceStarter: "price_starter_test"}
	ev, err := c.ParseWebhook([]byte(payload), sign(t, payload, testSecret))
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if ev.Kind != billing.KindSubscription {
		t.Errorf("kind = %v, want KindSubscription", ev.Kind)
	}
	if ev.Status != "active" {
		t.Errorf("status = %q, want active", ev.Status)
	}
	if want := time.Unix(periodEndUnix, 0).UTC(); !ev.PeriodEnd.Equal(want) {
		t.Errorf("period end = %v, want %v (read from the subscription ITEM)", ev.PeriodEnd, want)
	}
	if ev.Plan != plan.Starter {
		t.Errorf("plan = %v, want starter", ev.Plan)
	}
}

// TestParseWebhookRejectsBadSignature keeps the security property the version fix
// must not erode: a forged payload is still refused, and refused AS a signature
// failure rather than the misconfiguration error.
func TestParseWebhookRejectsBadSignature(t *testing.T) {
	payload := fmt.Sprintf(`{"id":"evt_x","object":"event","api_version":%q,"type":"ping","data":{"object":{}}}`, stripe.APIVersion)

	c := &Client{webhookSecret: testSecret}
	_, err := c.ParseWebhook([]byte(payload), sign(t, payload, "whsec_a_different_secret_000000000000000"))
	if err == nil {
		t.Fatal("ParseWebhook accepted a payload signed with the wrong secret")
	}
	if errors.Is(err, billing.ErrWebhookNotSignature) {
		t.Errorf("a bad signature was reported as a non-signature failure: %v", err)
	}
}

// TestParseWebhookVersionMismatchIsNotReportedAsSignature covers the mislabeling
// that made the original outage so expensive to diagnose: an endpoint on a
// different API version must not surface as "invalid webhook signature", or an
// operator burns hours rotating a secret that was never wrong.
func TestParseWebhookVersionMismatchIsNotReportedAsSignature(t *testing.T) {
	payload := `{"id":"evt_old","object":"event","api_version":"2019-01-01","type":"ping","data":{"object":{}}}`

	c := &Client{webhookSecret: testSecret}
	_, err := c.ParseWebhook([]byte(payload), sign(t, payload, testSecret))
	if err == nil {
		t.Fatal("ParseWebhook accepted an event from a mismatched API version")
	}
	if !errors.Is(err, billing.ErrWebhookNotSignature) {
		t.Errorf("version mismatch not tagged as a non-signature failure, so it would be\n"+
			"reported to the operator as a bad secret; got: %v", err)
	}
}
