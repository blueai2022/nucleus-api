package service

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/blueai2022/nucleus/internal/session"
	nucleusv1 "github.com/blueai2022/nucleus/pkg/nucleus/v1"
	"github.com/blueai2022/nucleus/pkg/nucleus/v1/nucleusv1connect"
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

	if _, err := s.sessions.Create(ctx, projectID, reqCode); err != nil {
		reason := nucleusv1.ErrorReason_ERROR_REASON_INTERNAL
		msg := "failed to validate session"

		if errors.Is(err, session.ErrProjectNotFound) {
			reason = nucleusv1.ErrorReason_ERROR_REASON_PROJECT_NOT_FOUND
			msg = fmt.Sprintf("project %q not found", projectID)
		} else if errors.Is(err, session.ErrRequirementNotFound) {
			reason = nucleusv1.ErrorReason_ERROR_REASON_REQUIREMENT_NOT_FOUND
			msg = fmt.Sprintf("requirement %q not found", reqCode)
		}

		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: reason,
			Message:     msg,
		}), nil
	}

	chatSession, err := s.chatManager.Create(ctx, projectID, reqCode)
	if err != nil {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INTERNAL,
			Message:     fmt.Sprintf("failed to create chat session: %v", err),
		}), nil
	}
	defer func() {
		if err := s.chatManager.Close(projectID, reqCode); err != nil {
			// TODO: Log the error but don't fail the request
		}
	}()

	// Send request to Claude
	prompt := fmt.Sprintf("Generate starter implementation for requirement: %s", reqCode)
	response, err := chatSession.Chat(ctx, prompt)
	if err != nil {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INTERNAL,
			Message:     fmt.Sprintf("failed to get implementation: %v", err),
		}), nil
	}

	return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
		Status:         nucleusv1.Status_STATUS_SUCCESS,
		Implementation: response,
		Message:        "starter implementation generated",
	}), nil
}
