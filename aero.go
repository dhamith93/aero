package aero

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dhamith93/aero/device"
	"github.com/dhamith93/aero/file"
	"github.com/dhamith93/aero/internal/api"
	"github.com/dhamith93/aero/internal/auth"
	socketserver "github.com/dhamith93/aero/internal/socket_server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Aero struct {
	key          string
	Devices      []device.Device
	Self         *device.Device
	Server       api.Server
	socketServer socketserver.SocketServer
	grpcServer   *grpc.Server
	IsMaster     bool
}

func New(device device.Device, isMaster bool) Aero {
	aero := Aero{}
	aero.Devices = append(aero.Devices, device)
	aero.Self = &aero.Devices[0]
	aero.IsMaster = isMaster
	return aero
}

func (aero *Aero) SetKey(key string) {
	aero.key = key
}

func (aero *Aero) StartGrpcServer() error {
	if len(aero.key) == 0 {
		return fmt.Errorf("auth key is not set")
	}
	aero.Server = api.Server{IsMaster: aero.IsMaster}
	aero.Server.Devices = append(aero.Server.Devices, device.GenerateAPIDeviceFromDevice(aero.Self))
	aero.Server.Self = aero.Server.Devices[0]
	aero.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(aero.authInterceptor))
	api.RegisterServiceServer(aero.grpcServer, &aero.Server)
	lis, err := net.Listen("tcp", ":"+aero.Self.Port)
	if err != nil {
		return err
	}
	return aero.grpcServer.Serve(lis)
}

func (aero *Aero) StartSocketServer() error {
	if len(aero.key) == 0 {
		return fmt.Errorf("auth key is not set")
	}
	aero.socketServer = socketserver.SocketServer{Port: aero.Server.Self.SocketPort, Devices: &aero.Server.Devices, Self: aero.Self}
	return aero.socketServer.Start()
}

func (aero *Aero) Stop() {
	aero.grpcServer.Stop()
	aero.socketServer.Stop()
}

func (aero *Aero) AddFile(f file.File) error {
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
	aero.Self.Files[len(aero.Self.Files)-1] = file.File{}
	aero.Self.Files = aero.Self.Files[:len(aero.Self.Files)-1]
	aero.SendRefresh(*aero.Self)
	return nil
}

func (aero *Aero) SendInit(d device.Device, master device.Device) ([]device.Device, error) {
	aero.Self = &d
	device := device.GenerateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.initDevice(device, master)
}

func (aero *Aero) SendRefresh(d device.Device) (device.Device, error) {
	aero.Self = &d
	device := device.GenerateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.refreshDevice(device)
}

func (aero *Aero) GetList() ([]device.Device, error) {
	return aero.getList()
}

func (aero *Aero) GetStatus(d device.Device) (device.Device, error) {
	return aero.getStatus(d)
}

func (aero *Aero) FetchFile(d device.Device, fileIdx int) error {
	return aero.fetchFile(d, fileIdx)
}

func (aero *Aero) RequestFile(d device.Device, fileIdx int) error {
	return aero.socketServer.RequestFile(d, fileIdx)
}

func (aero *Aero) initDevice(d *api.Device, master device.Device) ([]device.Device, error) {
	conn, c, ctx, cancel, err := aero.createClient(master)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer cancel()
	out := make([]device.Device, 0)
	data, err := c.Init(ctx, d)
	if err != nil {
		return nil, err
	}
	for _, d := range data.Devices {
		out = append(out, *device.GenerateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out, nil
}

func (aero *Aero) refreshDevice(d *api.Device) (device.Device, error) {
	out := device.Device{}
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

	out = *device.GenerateDeviceFromAPIDevice(dev)
	return out, nil
}

func (aero *Aero) getList() ([]device.Device, error) {
	conn, c, ctx, cancel, err := aero.createClient(aero.Devices[0])
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer cancel()
	out := make([]device.Device, 0)
	data, err := c.List(ctx, &api.Void{})
	if err != nil {
		return nil, err
	}
	for _, d := range data.Devices {
		out = append(out, *device.GenerateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out, nil
}

func (aero *Aero) getStatus(d device.Device) (device.Device, error) {
	out := device.Device{}
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

	out = *device.GenerateDeviceFromAPIDevice(dev)
	return out, nil
}

func (aero *Aero) fetchFile(d device.Device, fileIdx int) error {
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

func (aero *Aero) createClient(d device.Device) (*grpc.ClientConn, api.ServiceClient, context.Context, context.CancelFunc, error) {
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
