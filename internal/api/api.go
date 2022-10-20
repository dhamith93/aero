package api

import (
	context "context"
	"fmt"
)

type Server struct {
	Devices  []*Device
	Self     *Device
	Listener *chan bool
	IsMaster bool
}

func (s *Server) Init(ctx context.Context, in *Device) (*Devices, error) {
	if !s.IsMaster {
		return nil, fmt.Errorf("node is not master")
	}
	s.Devices = append(s.Devices, in)
	devices := make([]*Device, 0)
	for i := range s.Devices {
		devices = append(devices, s.Devices[i])
	}
	*s.Listener <- true
	return &Devices{Devices: devices}, nil
}

func (s *Server) Refresh(ctx context.Context, in *Device) (*Device, error) {
	if !s.IsMaster {
		return nil, fmt.Errorf("node is not master")
	}
	out := Device{}
	for i := range s.Devices {
		if s.Devices[i].Hash == in.Hash {
			s.Devices[i].Files = in.Files
			s.Devices[i].Active = in.Active
			*s.Listener <- true
			return s.Devices[i], nil
		}
	}
	return &out, fmt.Errorf("did not find a matching device")
}

func (s *Server) List(ctx context.Context, in *Void) (*Devices, error) {
	devices := make([]*Device, 0)
	for i := range s.Devices {
		devices = append(devices, s.Devices[i])
	}
	return &Devices{Devices: devices}, nil
}

func (s *Server) Status(ctx context.Context, in *Void) (*Device, error) {
	return s.Self, nil
}

func (s *Server) Fetch(ctx context.Context, in *File) (*FetchResponse, error) {
	for _, f := range s.Self.Files {
		if f.Hash == in.Hash {
			return &FetchResponse{Success: true, Error: ""}, nil
		}
	}

	return &FetchResponse{Success: false, Error: "file not found"}, nil
}
