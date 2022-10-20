package aero

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dhamith93/aero/internal/api"
	"github.com/dhamith93/aero/internal/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Aero struct {
	key          string
	Devices      []Device
	Self         *Device
	Server       api.Server
	SocketServer SocketServer
	grpcServer   *grpc.Server
	Listener     chan bool
	IsMaster     bool
}

func New(device Device, isMaster bool) Aero {
	aero := Aero{}
	aero.Devices = append(aero.Devices, device)
	aero.Self = &aero.Devices[0]
	aero.IsMaster = isMaster
	aero.Listener = make(chan bool)
	return aero
}

func (aero *Aero) SetKey(key string) {
	aero.key = key
}

func (aero *Aero) StartGrpcServer() error {
	if len(aero.key) == 0 {
		return fmt.Errorf("auth key is not set")
	}
	aero.Server = api.Server{IsMaster: aero.IsMaster, Listener: &aero.Listener}
	aero.Server.Devices = append(aero.Server.Devices, GenerateAPIDeviceFromDevice(aero.Self))
	aero.Server.Self = aero.Server.Devices[0]
	aero.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(aero.authInterceptor))
	api.RegisterServiceServer(aero.grpcServer, &aero.Server)
	lis, err := net.Listen("tcp", ":"+aero.Self.Port)
	if err != nil {
		return err
	}
	go aero.listenForDeviceChanges()
	return aero.grpcServer.Serve(lis)
}

func (aero *Aero) StartSocketServer() error {
	if len(aero.key) == 0 {
		return fmt.Errorf("auth key is not set")
	}
	aero.SocketServer = SocketServer{Port: aero.Server.Self.SocketPort, Devices: &aero.Devices, Self: aero.Self, Messages: &AeroMessages{}}
	return aero.SocketServer.Start()
}

func (aero *Aero) Stop() {
	aero.grpcServer.Stop()
	aero.SocketServer.Stop()
}

func (aero *Aero) listenForDeviceChanges() {
	for v := range aero.Listener {
		if v {
			out := []Device{}
			for _, d := range aero.Server.Devices {
				out = append(out, *GenerateDeviceFromAPIDevice(d))
			}
			aero.Devices = out
		}
	}
	aero.Listener <- true
}

func (aero *Aero) AddFile(f File) error {
	for _, file := range aero.Self.Files {
		if f.Hash == file.Hash {
			return fmt.Errorf("file with same hash exists")
		}
	}
	aero.Self.Files = append(aero.Self.Files, f)
	aero.SendRefresh(*aero.Self)
	return nil
}

func (aero *Aero) RemoveFileAt(fileIdx int) error {
	if fileIdx < 0 && len(aero.Self.Files) >= fileIdx {
		return fmt.Errorf("file index out of bound")
	}
	aero.Self.Files[fileIdx] = aero.Self.Files[len(aero.Self.Files)-1]
	aero.Self.Files[len(aero.Self.Files)-1] = File{}
	aero.Self.Files = aero.Self.Files[:len(aero.Self.Files)-1]
	aero.SendRefresh(*aero.Self)
	return nil
}

func (aero *Aero) SendInit(d Device, master Device) ([]Device, error) {
	aero.Self = &d
	device := GenerateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.initDevice(device, master)
}

func (aero *Aero) SendRefresh(d Device) (Device, error) {
	aero.Self = &d
	device := GenerateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.refreshDevice(device)
}

func (aero *Aero) GetList() ([]Device, error) {
	return aero.getList()
}

func (aero *Aero) GetStatus(d Device) (Device, error) {
	return aero.getStatus(d)
}

func (aero *Aero) FetchFile(d Device, fileIdx int) error {
	return aero.fetchFile(d, fileIdx)
}

func (aero *Aero) Download(d Device, fileIdx int) int {
	return aero.SocketServer.Download(d, fileIdx)
}

func (aero *Aero) initDevice(d *api.Device, master Device) ([]Device, error) {
	conn, c, ctx, cancel, err := aero.createClient(master)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer cancel()
	out := make([]Device, 0)
	data, err := c.Init(ctx, d)
	if err != nil {
		return nil, err
	}
	for _, d := range data.Devices {
		out = append(out, *GenerateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out, nil
}

func (aero *Aero) refreshDevice(d *api.Device) (Device, error) {
	out := Device{}
	conn, c, ctx, cancel, err := aero.createClient(aero.Devices[0])
	if err != nil {
		return out, err
	}
	defer conn.Close()
	defer cancel()

	dev, err := c.Refresh(ctx, d)
	if err != nil {
		return out, err
	}

	out = *GenerateDeviceFromAPIDevice(dev)
	return out, nil
}

func (aero *Aero) getList() ([]Device, error) {
	conn, c, ctx, cancel, err := aero.createClient(aero.Devices[0])
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer cancel()
	out := make([]Device, 0)
	data, err := c.List(ctx, &api.Void{})
	if err != nil {
		return nil, err
	}
	for _, d := range data.Devices {
		out = append(out, *GenerateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out, nil
}

func (aero *Aero) getStatus(d Device) (Device, error) {
	out := Device{}
	conn, c, ctx, cancel, err := aero.createClient(d)
	if err != nil {
		return out, err
	}
	defer conn.Close()
	defer cancel()

	dev, err := c.Status(ctx, &api.Void{})
	if err != nil {
		return out, err
	}

	out = *GenerateDeviceFromAPIDevice(dev)
	return out, nil
}

func (aero *Aero) fetchFile(d Device, fileIdx int) error {
	conn, c, ctx, cancel, err := aero.createClient(d)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer cancel()

	if fileIdx < 0 || fileIdx >= len(d.Files) {
		return fmt.Errorf("file doesn't exists in the device")
	}

	resp, err := c.Fetch(ctx, &api.File{Hash: d.Files[fileIdx].Hash})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

func (aero *Aero) createClient(d Device) (*grpc.ClientConn, api.ServiceClient, context.Context, context.CancelFunc, error) {
	var (
		conn *grpc.ClientConn
		err  error
	)

	conn, err = grpc.Dial(d.Ip+":"+d.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, nil, nil, nil, err
	}
	c := api.NewServiceClient(conn)
	token := aero.generateToken()
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{"jwt": token})), time.Second*10)
	return conn, c, ctx, cancel, nil
}

func (aero *Aero) authInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "INTERNAL_SERVER_ERROR")
	}
	if len(meta["jwt"]) != 1 {
		return nil, status.Error(codes.Unauthenticated, "token empty")
	}
	if !auth.ValidToken(meta["jwt"][0], aero.key) {
		return nil, status.Error(codes.PermissionDenied, "invalid auth token")
	}
	return handler(ctx, req)
}

func (aero *Aero) generateToken() string {
	token, err := auth.GenerateJWT(aero.key)
	if err != nil {
		return ""
	}
	return token
}
