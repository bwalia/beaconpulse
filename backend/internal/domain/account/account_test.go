package account

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

type fakeRepo struct {
	user    *auth.User
	deleted bool
}

func (f *fakeRepo) GetUserByID(context.Context, uuid.UUID) (*auth.User, error) { return f.user, nil }
func (f *fakeRepo) DeleteOrganization(context.Context, uuid.UUID) error {
	f.deleted = true
	return nil
}

type fakeBilling struct{ err error }

func (f fakeBilling) CloseAccount(context.Context, uuid.UUID) error { return f.err }

type fakeSyncer struct{ called bool }

func (f *fakeSyncer) Sync(context.Context) error { f.called = true; return nil }

func owner() *auth.User {
	return &auth.User{ID: uuid.New(), OrgID: uuid.New(), Email: "jane@example.com", Role: auth.RoleOwner}
}

func TestDelete_ErasesOrgAndResyncs(t *testing.T) {
	u := owner()
	repo, sync := &fakeRepo{user: u}, &fakeSyncer{}
	if err := NewService(repo, fakeBilling{}, sync).Delete(context.Background(), u.ID, u.OrgID, " JANE@example.com "); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !repo.deleted || !sync.called {
		t.Fatalf("want org deleted and control plane resynced, got deleted=%v synced=%v", repo.deleted, sync.called)
	}
}

func TestDelete_WrongConfirmationDeletesNothing(t *testing.T) {
	u := owner()
	repo := &fakeRepo{user: u}
	err := NewService(repo, fakeBilling{}, &fakeSyncer{}).Delete(context.Background(), u.ID, u.OrgID, "someone@else.com")
	if !apperror.IsCode(err, apperror.CodeValidation) || repo.deleted {
		t.Fatalf("want validation error and nothing deleted, got err=%v deleted=%v", err, repo.deleted)
	}
}

func TestDelete_BillingFailureAborts(t *testing.T) {
	u := owner()
	repo := &fakeRepo{user: u}
	err := NewService(repo, fakeBilling{err: errors.New("stripe down")}, &fakeSyncer{}).Delete(context.Background(), u.ID, u.OrgID, u.Email)
	if err == nil || repo.deleted {
		t.Fatalf("a live subscription must block deletion, got err=%v deleted=%v", err, repo.deleted)
	}
}
