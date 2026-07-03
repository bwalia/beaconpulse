// Package project is the bounded context for projects — logical groupings of
// monitored resources within an organization. It owns the Project aggregate and
// its use cases, depending only on the Repository interface for persistence and
// an audit.Recorder for the change log.
package project

import (
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
)

// Environment classifies a project's deployment stage. It doubles as a label
// propagated to Prometheus/alert routing so alerts can be prioritized by
// environment.
type Environment string

const (
	EnvProduction  Environment = "production"
	EnvStaging     Environment = "staging"
	EnvDevelopment Environment = "development"
)

// Valid reports whether e is a known environment.
func (e Environment) Valid() bool {
	switch e {
	case EnvProduction, EnvStaging, EnvDevelopment:
		return true
	default:
		return false
	}
}

// Project is a named container for monitors, scoped to one organization.
type Project struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	Slug        string
	Description string
	Environment Environment
	IsActive    bool
	CreatedBy   *uuid.UUID
	UpdatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Actor is the authenticated caller performing an operation. Services use it for
// org scoping, authorization, and audit attribution.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}
