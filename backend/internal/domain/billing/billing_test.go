package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
)

type fakeStore struct{ plan plan.Plan }

func (f *fakeStore) Plan(_ context.Context, _ uuid.UUID) (plan.Plan, error) { return f.plan, nil }
func (f *fakeStore) SetPlan(_ context.Context, _ uuid.UUID, p plan.Plan) error {
	f.plan = p
	return nil
}

type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ audit.Entry) error { return nil }

func TestChangePlanOwnerSucceeds(t *testing.T) {
	store := &fakeStore{plan: plan.Free}
	svc := NewService(store, noopRecorder{})
	actor := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleOwner}

	got, err := svc.ChangePlan(context.Background(), actor, plan.Pro)
	if err != nil {
		t.Fatalf("ChangePlan: %v", err)
	}
	if got != plan.Pro || store.plan != plan.Pro {
		t.Fatalf("plan not changed: got=%s store=%s", got, store.plan)
	}
}

func TestChangePlanRejectsNonAdmin(t *testing.T) {
	svc := NewService(&fakeStore{plan: plan.Free}, noopRecorder{})
	actor := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleMember}
	if _, err := svc.ChangePlan(context.Background(), actor, plan.Pro); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("expected forbidden for member, got %v", err)
	}
}

func TestChangePlanRejectsUnknownPlan(t *testing.T) {
	svc := NewService(&fakeStore{plan: plan.Free}, noopRecorder{})
	actor := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleOwner}
	if _, err := svc.ChangePlan(context.Background(), actor, plan.Plan("enterprise")); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error for unknown plan, got %v", err)
	}
}

func TestCatalogHasThreePlans(t *testing.T) {
	svc := NewService(&fakeStore{}, noopRecorder{})
	if len(svc.Catalog()) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(svc.Catalog()))
	}
}
