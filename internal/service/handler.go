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
) (
	*connect.Response[nucleusv1.GetStarterImplementationResponse],
	error,
) {
	projectID := req.Msg.GetProjectId()
	reqCode := req.Msg.GetRequirementCode()

	if projectID == "" || reqCode == "" {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_INVALID_INPUT,
			Message:     "project_id and requirement_code are required",
		}), nil
	}

	if _, err := s.sessions.Session(ctx, projectID, reqCode); err != nil {
		reason := nucleusv1.ErrorReason_ERROR_REASON_INTERNAL
		msg := "failed to validate session"

		switch {
		case errors.Is(err, session.ErrProjectNotFound):
			reason = nucleusv1.ErrorReason_ERROR_REASON_PROJECT_NOT_FOUND
			msg = fmt.Sprintf("project %q not found", projectID)
		case errors.Is(err, session.ErrRequirementNotFound):
			reason = nucleusv1.ErrorReason_ERROR_REASON_REQUIREMENT_NOT_FOUND
			msg = fmt.Sprintf("requirement %q not found", reqCode)
		}

		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: reason,
			Message:     msg,
		}), nil
	}

	// TODO: generate real starter implementation
	markdown := fmt.Sprintf("```\n// Starter implementation for %s\n```", reqCode)

	return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
		Status:         nucleusv1.Status_STATUS_SUCCESS,
		Implementation: markdown,
		Message:        "starter implementation generated",
	}), nil
}
