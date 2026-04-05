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
	s := &Service{}

	s.connectPath, s.connectHandler = nucleusv1connect.NewNucleusServiceHandler(s, opts...)

	vSvc := vanguard.NewService(nucleusv1connect.NucleusServiceName, s.connectHandler)
	vHandler, err := vanguard.NewTranscoder([]*vanguard.Service{vSvc})
	if err != nil {
		return nil, err
	}
	s.vanguardHandler = vHandler

	return s, nil
}

func (s *Service) ConnectHandler() (string, http.Handler) {
	return s.connectPath, s.connectHandler
}

func (s *Service) VanguardHandler() (string, http.Handler) {
	return "/", s.vanguardHandler
}
