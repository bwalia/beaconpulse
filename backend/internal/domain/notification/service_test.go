package notification

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
)

type fakeChannelRepo struct {
	channels map[uuid.UUID]*Channel
}

func newFakeChannelRepo() *fakeChannelRepo {
	return &fakeChannelRepo{channels: map[uuid.UUID]*Channel{}}
}

func (f *fakeChannelRepo) Create(_ context.Context, c *Channel) error {
	f.channels[c.ID] = c
	return nil
}
func (f *fakeChannelRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (*Channel, error) {
	c, ok := f.channels[id]
	if !ok || c.OrgID != orgID {
		return nil, apperror.NotFound("not found")
	}
	return c, nil
}
func (f *fakeChannelRepo) List(_ context.Context, orgID uuid.UUID, _ ListFilter) ([]Channel, int, error) {
	items := f.byOrg(orgID)
	return items, len(items), nil
}
func (f *fakeChannelRepo) ListEnabledByOrg(_ context.Context, orgID uuid.UUID) ([]Channel, error) {
	var out []Channel
	for _, c := range f.byOrg(orgID) {
		if c.Enabled {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeChannelRepo) byOrg(orgID uuid.UUID) []Channel {
	var out []Channel
	for _, c := range f.channels {
		if c.OrgID == orgID {
			out = append(out, *c)
		}
	}
	return out
}
func (f *fakeChannelRepo) Update(_ context.Context, c *Channel) error {
	f.channels[c.ID] = c
	return nil
}
func (f *fakeChannelRepo) SoftDelete(_ context.Context, _, id, _ uuid.UUID) error {
	delete(f.channels, id)
	return nil
}
func (f *fakeChannelRepo) FindByType(_ context.Context, orgID uuid.UUID, t ChannelType) (*Channel, error) {
	for _, c := range f.channels {
		if c.OrgID == orgID && c.Type == t {
			return c, nil
		}
	}
	return nil, nil
}

type fakeNotifier struct {
	called bool
	secret string
	msg    Message
	err    error
}

func (f *fakeNotifier) Type() ChannelType { return TypeTelegram }
func (f *fakeNotifier) Send(_ context.Context, ch Decrypted, msg Message) error {
	f.called = true
	f.secret = ch.Secret
	f.msg = msg
	return f.err
}

type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ audit.Entry) error { return nil }

func newTestService(t *testing.T, notif Notifier) *Service {
	t.Helper()
	key := make([]byte, 32)
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	registry := map[ChannelType]Notifier{TypeTelegram: notif}
	return NewService(newFakeChannelRepo(), cipher, registry, noopRecorder{}, "http://dash")
}

func writerActor() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleAdmin}
}

func TestCreateTelegramEncryptsSecret(t *testing.T) {
	svc := newTestService(t, &fakeNotifier{})
	ch, err := svc.Create(context.Background(), writerActor(), CreateInput{
		Name:   "Ops",
		Type:   TypeTelegram,
		Config: map[string]string{"chat_id": "123"},
		Secret: "bot-token-secret",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !ch.HasSecret() {
		t.Fatal("expected channel to have a secret")
	}
	if ch.SecretEncrypted == "bot-token-secret" {
		t.Fatal("secret was stored in plaintext")
	}
}

func TestCreateRejectsUnsupportedType(t *testing.T) {
	svc := newTestService(t, &fakeNotifier{})
	_, err := svc.Create(context.Background(), writerActor(), CreateInput{
		Name: "x", Type: TypeSlack, Config: map[string]string{}, Secret: "y",
	})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestEnsureAPNsChannelIsIdempotentAndRespectsMuting(t *testing.T) {
	repo := newFakeChannelRepo()
	cipher, err := crypto.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, cipher, map[ChannelType]Notifier{}, noopRecorder{}, "http://dash")
	orgID := uuid.New()
	ctx := context.Background()

	// First device registration creates an enabled channel.
	if err := svc.EnsureAPNsChannel(ctx, orgID); err != nil {
		t.Fatalf("first EnsureAPNsChannel: %v", err)
	}
	ch, _ := repo.FindByType(ctx, orgID, TypeAPNs)
	if ch == nil || !ch.Enabled {
		t.Fatalf("expected an enabled apns channel, got %+v", ch)
	}

	// The user then mutes push.
	ch.Enabled = false

	// A later registration must neither re-enable it nor create a duplicate.
	if err := svc.EnsureAPNsChannel(ctx, orgID); err != nil {
		t.Fatalf("second EnsureAPNsChannel: %v", err)
	}
	if again, _ := repo.FindByType(ctx, orgID, TypeAPNs); again.Enabled {
		t.Error("a muted push channel must not be re-enabled on re-registration")
	}
	n := 0
	for _, c := range repo.channels {
		if c.OrgID == orgID && c.Type == TypeAPNs {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected exactly one apns channel, got %d", n)
	}
}

func TestCreateTelegramRequiresChatID(t *testing.T) {
	svc := newTestService(t, &fakeNotifier{})
	_, err := svc.Create(context.Background(), writerActor(), CreateInput{
		Name: "x", Type: TypeTelegram, Config: map[string]string{}, Secret: "token",
	})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error for missing chat_id, got %v", err)
	}
}

func TestSendTestDecryptsAndDelivers(t *testing.T) {
	notif := &fakeNotifier{}
	svc := newTestService(t, notif)
	actor := writerActor()
	ch, err := svc.Create(context.Background(), actor, CreateInput{
		Name: "Ops", Type: TypeTelegram, Config: map[string]string{"chat_id": "123"}, Secret: "my-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendTest(context.Background(), actor, ch.ID); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	if !notif.called {
		t.Fatal("notifier was not called")
	}
	if notif.secret != "my-token" {
		t.Errorf("notifier received secret %q, want decrypted 'my-token'", notif.secret)
	}
	if !notif.msg.IsTest {
		t.Error("expected IsTest message")
	}
}
