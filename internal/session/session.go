package session

import (
	"context"
	"errors"
)

var (
	ErrProjectNotFound     = errors.New("project not found")
	ErrRequirementNotFound = errors.New("requirement not found")
)

// Session represents an active code session.
type Session struct {
	ProjectID       string
	RequirementCode string
}

// Manager retrieves and validates active code sessions.
type Manager interface {
	// Session retrieves an active code session for the given project and requirement.
	Session(ctx context.Context, projectID, requirementCode string) (*Session, error)
}

// manager implements Manager.
type manager struct{}

func NewManager() Manager {
	return &manager{}
}

func (m *manager) Session(ctx context.Context, projectID, requirementCode string) (*Session, error) {
	// TODO: retrieve active session for projectID + requirementCode
	return nil, ErrRequirementNotFound
}