package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"beacon/internal/adapter/fcm"
	"beacon/internal/domain/device"
	"beacon/internal/domain/notification"
	"beacon/internal/platform/logger"
)

// fcmSender is the slice of the FCM client the notifier uses.
type fcmSender interface {
	Send(ctx context.Context, body []byte) (fcm.Result, error)
}

// FCMNotifier delivers alerts to Android devices via Firebase Cloud Messaging.
// Like APNs it has no per-channel secret: the service account is platform config
// and the destinations are the org's registered android tokens.
type FCMNotifier struct {
	client fcmSender
	store  device.TokenStore
}

func NewFCMNotifier(client *fcm.Client, store device.TokenStore) *FCMNotifier {
	return &FCMNotifier{client: client, store: store}
}

func (n *FCMNotifier) Type() notification.ChannelType { return notification.TypeFCM }

// androidChannelID is the notification channel the Android app creates for alerts.
const androidChannelID = "alerts"

// fcmMessage is the FCM v1 "message". notification drives the visible banner;
// data carries the same deep-link keys as the APNs payload (FCM data values must
// be strings).
type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
	Android      fcmAndroid        `json:"android"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroid struct {
	Priority     string `json:"priority"`
	Notification struct {
		ChannelID string `json:"channel_id"`
	} `json:"notification"`
}

// Send fans the message out to every android token registered for the channel's
// org. Best-effort per token, like APNs: one failure never blocks the rest, and a
// token FCM declares dead is pruned.
func (n *FCMNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	log := logger.FromContext(ctx)
	if ch.OrgID == uuid.Nil {
		return fmt.Errorf("fcm: channel has no org id")
	}
	tokens, err := n.store.TokensByOrg(ctx, ch.OrgID, device.PlatformAndroid)
	if err != nil {
		return fmt.Errorf("fcm: load device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	m := buildFCMMessage(msg)
	var (
		delivered int
		lastErr   error
	)
	for _, tok := range tokens {
		m.Token = tok
		body, err := json.Marshal(map[string]fcmMessage{"message": m})
		if err != nil {
			return fmt.Errorf("fcm: marshal message: %w", err)
		}
		res, err := n.client.Send(ctx, body)
		switch {
		case err != nil:
			lastErr = err
			log.Warn("fcm: send failed", slog.String("error", err.Error()))
		case res.OK():
			delivered++
		case res.TokenIsDead():
			if derr := n.store.Delete(ctx, tok); derr != nil {
				log.Warn("fcm: prune dead token failed", slog.String("error", derr.Error()))
			}
		default:
			lastErr = fmt.Errorf("fcm: %d %s %s", res.StatusCode, res.Code, res.Message)
			log.Warn("fcm: send rejected",
				slog.Int("status", res.StatusCode), slog.String("code", res.Code), slog.String("message", res.Message))
		}
	}
	if delivered == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

// buildFCMMessage renders a Message the same way the APNs payload does (title,
// body, monitor id + status for deep-linking), minus the per-device token.
func buildFCMMessage(msg notification.Message) fcmMessage {
	m := fcmMessage{
		Notification: fcmNotification{Title: apnsTitle(msg), Body: apnsBody(msg)},
		Data:         map[string]string{},
		Android:      fcmAndroid{Priority: "high"},
	}
	m.Android.Notification.ChannelID = androidChannelID
	if msg.MonitorID != "" {
		m.Data["monitor_id"] = msg.MonitorID
	}
	if msg.Status != "" {
		m.Data["status"] = string(msg.Status)
	}
	return m
}
