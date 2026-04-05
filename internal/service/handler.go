package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
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

	// TODO: look up project — replace with real store call
	projectExists := false // placeholder
	if !projectExists {
		return connect.NewResponse(&nucleusv1.GetStarterImplementationResponse{
			Status:      nucleusv1.Status_STATUS_ERROR,
			ErrorReason: nucleusv1.ErrorReason_ERROR_REASON_PROJECT_NOT_FOUND,
			Message:     fmt.Sprintf("project %q not found", projectID),
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
