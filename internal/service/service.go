package service

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"github.com/blueai2022/nucleus/internal/session"
	"github.com/blueai2022/nucleus/pkg/nucleus/v1/nucleusv1connect"
)

type Service struct {
	sessions        session.Manager
	connectPath     string
	connectHandler  http.Handler
	vanguardHandler http.Handler
}

func New(sessions session.Manager, opts ...connect.HandlerOption) (*Service, error) {
	svc := &Service{sessions: sessions}

	path, handler := nucleusv1connect.NewNucleusServiceHandler(svc, opts...)
	svc.connectPath = path
	svc.connectHandler = handler

	vanguardService := vanguard.NewService(
		nucleusv1connect.NucleusServiceName,
		svc.connectHandler,
	)
	vanguardHandler, err := vanguard.NewTranscoder(
		[]*vanguard.Service{vanguardService},
	)
	if err != nil {
		return nil, err
	}

	svc.vanguardHandler = vanguardHandler

	return svc, nil
}

func (svc *Service) ConnectHandler() (string, http.Handler) {
	return svc.connectPath, svc.connectHandler
}

func (svc *Service) VanguardHandler() (string, http.Handler) {
	return "/", svc.vanguardHandler
}
