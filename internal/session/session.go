package session

import (
	"context"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

var (
	ErrProjectNotFound     = errors.New("project not found")
	ErrRequirementNotFound = errors.New("requirement not found")
	ErrSessionNotFound     = errors.New("session not found")
)

// Manager manages Claude Code sessions for code generation
type Manager interface {
	// CreateSession creates a new Claude Code session with composed workspace
	CreateSession(ctx context.Context, config SessionConfig) (*ClaudeCodeSession, error)

	// Session retrieves an existing session
	Session(projectID, requirementCode string) (*ClaudeCodeSession, error)

	// CloseSession cleans up a session and its workspace
	CloseSession(projectID, requirementCode string) error
}

// SessionConfig contains parameters for creating a session
type SessionConfig struct {
	ProjectID            string
	RequirementCode      string
	Language             Language
	MainProjectPath      string
	TemplateRequirements []string
}

// Session represents an active code session.
type Session struct {
	ProjectID     string
	WorkspaceRoot string
	client        anthropic.Client
	tools         []anthropic.BetaTool
	messages      []anthropic.BetaMessageParam
}
