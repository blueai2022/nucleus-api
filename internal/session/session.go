package session

import (
	"context"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

var (
	ErrProjectNotFound     = errors.New("project not found")
	ErrRequirementNotFound = errors.New("requirement not found")
)

// Session represents an active code session.
type Session struct {
	ProjectID     string
	WorkspaceRoot string
	client        anthropic.Client
	tools         []anthropic.BetaTool
	messages      []anthropic.BetaMessageParam
}

// Manager retrieves and validates active code sessions.
type Manager interface {
	// Create creates a new code session for the given project and requirement.
	Create(ctx context.Context, projectID, requirementCode string) (*Session, error)

	// Close closes the session for the given project and requirement.
	Close(projectID, requirementCode string) error

	// Session retrieves an existing session
	Session(projectID, requirementCode string) (*Session, error)
}
