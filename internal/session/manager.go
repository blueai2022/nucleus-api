package session

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

type manager struct {
	workspaceRoot string
	templateRoot  string
	composer      WorkspaceComposer
	sessions      map[string]*ClaudeCodeSession
}

// NewManager creates a new session manager
func NewManager(workspaceRoot, templateRoot string) Manager {
	return &manager{
		workspaceRoot: workspaceRoot,
		templateRoot:  templateRoot,
		composer:      NewWorkspaceComposer(workspaceRoot, templateRoot),
		sessions:      make(map[string]*ClaudeCodeSession),
	}
}

// CreateSession creates a new Claude Code session with composed workspace
func (m *manager) CreateSession(ctx context.Context, config SessionConfig) (*ClaudeCodeSession, error) {
	sessionID := fmt.Sprintf("%s-%s", config.ProjectID, config.RequirementCode)

	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		log.Warn().
			Str("session_id", sessionID).
			Msg("session already exists, returning existing")
		return m.sessions[sessionID], nil
	}

	// Compose workspace with main project + templates
	log.Info().
		Str("session_id", sessionID).
		Str("project", config.MainProjectPath).
		Msg("composing workspace")

	workspacePath, err := m.composer.Compose(WorkspaceConfig{
		ProjectID:            config.ProjectID,
		Language:             config.Language,
		Type:                 ProjectTypeBackend, // TODO: make configurable
		MainProjectPath:      config.MainProjectPath,
		TemplateRequirements: config.TemplateRequirements,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compose workspace: %w", err)
	}

	// Create Claude Code session
	session, err := NewClaudeCodeSession(config.ProjectID, workspacePath)
	if err != nil {
		m.composer.Destroy(config.ProjectID)
		return nil, fmt.Errorf("failed to create claude code session: %w", err)
	}

	m.sessions[sessionID] = session

	log.Info().
		Str("session_id", sessionID).
		Str("workspace", workspacePath).
		Msg("session created")

	return session, nil
}

// Session retrieves an existing session
func (m *manager) Session(projectID, requirementCode string) (*ClaudeCodeSession, error) {
	sessionID := fmt.Sprintf("%s-%s", projectID, requirementCode)

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// CloseSession cleans up a session and its workspace
func (m *manager) CloseSession(projectID, requirementCode string) error {
	sessionID := fmt.Sprintf("%s-%s", projectID, requirementCode)

	_, exists := m.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// Cleanup workspace
	// if err := m.composer.Destroy(projectID); err != nil {
	// 	log.Warn().
	// 		Err(err).
	// 		Str("session_id", sessionID).
	// 		Msg("failed to cleanup workspace")
	// }

	delete(m.sessions, sessionID)

	log.Info().
		Str("session_id", sessionID).
		Msg("session closed")

	return nil
}
