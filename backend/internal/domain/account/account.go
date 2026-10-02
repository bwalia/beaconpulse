// Package account owns closing an account for good: the "delete my account" right
// (UK/EU GDPR erasure; also required of any app offering sign-up on the App Store).
//
// There are no team invites, so an organization has exactly one user — its owner —
// and deleting the account means erasing the whole organization.
package account

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// Repository erases an organization and everything it owns.
type Repository interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*auth.User, error)
	DeleteOrganization(ctx context.Context, orgID uuid.UUID) error
}

// Billing stops all charges for an org. Implemented by billing.Service.
type Billing interface {
	CloseAccount(ctx context.Context, orgID uuid.UUID) error
}

// Syncer regenerates the probe config, so deleted monitors stop being probed now
// rather than at the next reconcile. Implemented by the control-plane enqueuer.
type Syncer interface {
	Sync(ctx context.Context) error
}

// Service deletes accounts.
type Service struct {
	repo    Repository
	billing Billing // nil = billing not configured
	syncer  Syncer
}

// NewService builds a Service. billing may be nil.
func NewService(repo Repository, billing Billing, syncer Syncer) *Service {
	return &Service{repo: repo, billing: billing, syncer: syncer}
}

// Delete permanently erases the caller's account and organization. confirmEmail must
// match the account's email — a deliberate, typed confirmation that works for
// password and Google/Apple accounts alike.
//
// Order matters: billing is closed FIRST and a failure aborts, because deleting the
// org while its subscription lives on would keep charging a customer who has no
// account left to cancel it from.
func (s *Service) Delete(ctx context.Context, userID, orgID uuid.UUID, confirmEmail string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrgID != orgID {
		return apperror.Forbidden("you can only delete your own account")
	}
	// ponytail: one user per org today. When invites ship, a non-owner should delete
	// only their own user row instead of being refused.
	if user.Role != auth.RoleOwner {
		return apperror.Forbidden("only the account owner can delete the account")
	}
	if !strings.EqualFold(strings.TrimSpace(confirmEmail), user.Email) {
		return apperror.Validation("type your account email to confirm",
			apperror.FieldError{Field: "confirm_email", Message: "does not match your account email"})
	}

	if s.billing != nil {
		if err := s.billing.CloseAccount(ctx, orgID); err != nil {
			return apperror.New(apperror.CodeUnavailable,
				"couldn't cancel your subscription, so nothing was deleted — try again or contact support")
		}
	}
	if err := s.repo.DeleteOrganization(ctx, orgID); err != nil {
		return err
	}
	// Best-effort: the periodic reconcile drops the targets anyway if this fails.
	_ = s.syncer.Sync(ctx)
	return nil
}
