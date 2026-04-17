package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/blueai2022/nucleus/internal/session"
	nucleusv1 "github.com/blueai2022/nucleus/pkg/nucleus/v1"
	"github.com/blueai2022/nucleus/pkg/nucleus/v1/nucleusv1connect"
	"github.com/rs/zerolog/log"
)

// Ensure Handler implements the generated interface.
var _ nucleusv1connect.NucleusServiceHandler = (*Service)(nil)

func (s *Service) GetStarterImplementation(
	ctx context.Context,
	req *connect.Request[nucleusv1.GetStarterImplementationRequest],
) (*connect.Response[nucleusv1.GetStarterImplementationResponse], error) {
	projectID := req.Msg.GetProjectId()
	reqCode := req.Msg.GetRequirementCode()

	if projectID == "" || reqCode == "" {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INVALID_INPUT,
			Message:     "project_id and requirement_code are required",
		}), nil
	}

	// Lookup requirement metadata
	reqMeta, err := s.reqRegistry.Lookup(projectID, reqCode)
	if err != nil {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_REQUIREMENT_NOT_FOUND,
			Message:     fmt.Sprintf("requirement not found: %v", err),
		}), nil
	}

	// TODO: Get these from project registry/config
	mainProjectPath := "./test_fixtures/projects/my-service"
	templateReqs := []string{"metrics"}

	// Create session with metadata-derived language
	claudeSession, err := s.sessionManager.CreateSession(ctx, session.SessionConfig{
		ProjectID:            projectID,
		RequirementCode:      reqCode,
		Language:             reqMeta.Language, // From metadata
		MainProjectPath:      mainProjectPath,
		TemplateRequirements: templateReqs,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("project_id", projectID).
			Str("requirement_code", reqCode).
			Msg("failed to create session")

		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INTERNAL,
			Message:     fmt.Sprintf("failed to create session: %v", err),
		}), nil
	}
	defer func() {
		if err := s.sessionManager.CloseSession(projectID, reqCode); err != nil {
			log.Warn().
				Err(err).
				Str("project_id", projectID).
				Str("requirement_code", reqCode).
				Msg("failed to cleanup session")
		}
	}()

	// Generate code using Claude Code
	log.Info().
		Str("project_id", projectID).
		Str("requirement_code", reqCode).
		Msg("generating implementation")

	response, err := claudeSession.Generate(ctx, session.CodeGenerationRequest{
		// 		Prompt: fmt.Sprintf(`Generate implementation for requirement: %s

		// Read the requirement details and reference pkg/examples/ for PR-approved code patterns and styles.

		// Provide complete, production-ready code following the team's conventions.`, reqCode),
		// ExampleDirs: []string{"pkg/examples"},
		TargetFile:  reqMeta.TargetFile,     // From metadata
		ExampleDirs: reqMeta.ExampleDirs,    // From metadata
		Prompt:      reqMeta.PromptTemplate, // From metadata
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("project_id", projectID).
			Str("requirement_code", reqCode).
			Msg("code generation failed")

		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INTERNAL,
			Message:     fmt.Sprintf("failed to generate implementation: %v", err),
		}), nil
	}

	log.Info().
		Str("project_id", projectID).
		Str("requirement_code", reqCode).
		Int("code_length", len(response.Code)).
		Msg("implementation generated successfully")

	return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
		Status:         nucleusv1.Status_STATUS_SUCCESS,
		Implementation: response.Code,
		Message:        "implementation generated successfully",
	}), nil
}
