package service

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"github.com/blueai2022/nucleus/pkg/nucleus/v1/nucleusv1connect"
)

type Service struct {
	connectPath     string
	connectHandler  http.Handler
	vanguardHandler http.Handler
}

func New(opts ...connect.HandlerOption) (*Service, error) {
	svc := &Service{}

	svc.connectPath, svc.connectHandler = nucleusv1connect.NewNucleusServiceHandler(svc, opts...)

	vanguardService := vanguard.NewService(nucleusv1connect.NucleusServiceName, svc.connectHandler)
	vanguardHandler, err := vanguard.NewTranscoder([]*vanguard.Service{vanguardService})
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
