package aero

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/dhamith93/aero/device"
	"github.com/dhamith93/aero/file"
	"github.com/dhamith93/aero/internal/api"
	"github.com/dhamith93/aero/internal/auth"
	"github.com/dhamith93/aero/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Aero struct {
	Config   Config
	Devices  []device.Device
	Self     *device.Device
	Server   api.Server
	IsMaster bool
}

func New(config Config, device device.Device, isMaster bool) Aero {
	aero := Aero{
		Config: config,
	}
	aero.Devices = append(aero.Devices, device)
	aero.Self = &aero.Devices[0]
	aero.IsMaster = isMaster
	return aero
}

func (aero *Aero) Start() {
	var grpcServer *grpc.Server
	aero.Server = api.Server{IsMaster: aero.IsMaster}
	aero.Server.Devices = append(aero.Server.Devices, aero.generateAPIDeviceFromDevice(aero.Self))
	aero.Server.Self = aero.Server.Devices[0]
	grpcServer = grpc.NewServer(grpc.UnaryInterceptor(aero.authInterceptor))
	api.RegisterServiceServer(grpcServer, &aero.Server)
	lis, err := net.Listen("tcp", ":"+aero.Config.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}
}

func (aero *Aero) SendInit(d device.Device, master device.Device) []device.Device {
	aero.Self = &d
	device := aero.generateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.initDevice(device, master)
}

func (aero *Aero) SendRefresh(d device.Device) device.Device {
	aero.Self = &d
	device := aero.generateAPIDeviceFromDevice(&d)
	aero.Server.Self = device
	return aero.refreshDevice(device)
}

func (aero *Aero) GetList() []device.Device {
	return aero.getList()
}

func (aero *Aero) GetStatus(d device.Device) device.Device {
	return aero.getStatus(d)
}

func (aero *Aero) FetchFile(d device.Device, fileIdx int) (bool, string) {
	return aero.fetchFile(d, fileIdx)
}

func (aero *Aero) initDevice(d *api.Device, master device.Device) []device.Device {
	conn, c, ctx, cancel := aero.createClient(master)
	defer conn.Close()
	defer cancel()
	out := make([]device.Device, 0)
	data, err := c.Init(ctx, d)
	if err != nil {
		logger.Log("error", "error sending data: "+err.Error())
		return nil
	}
	for _, d := range data.Devices {
		out = append(out, *aero.generateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out
}

func (aero *Aero) refreshDevice(d *api.Device) device.Device {
	conn, c, ctx, cancel := aero.createClient(aero.Devices[0])
	out := device.Device{}
	defer conn.Close()
	defer cancel()

	device, err := c.Refresh(ctx, d)
	if err != nil {
		logger.Log("error", "error sending data: "+err.Error())
		return out
	}

	out = *aero.generateDeviceFromAPIDevice(device)
	return out
}

func (aero *Aero) getList() []device.Device {
	conn, c, ctx, cancel := aero.createClient(aero.Devices[0])
	defer conn.Close()
	defer cancel()
	out := make([]device.Device, 0)
	data, err := c.List(ctx, &api.Void{})
	if err != nil {
		logger.Log("error", "error sending data: "+err.Error())
		return nil
	}
	for _, d := range data.Devices {
		out = append(out, *aero.generateDeviceFromAPIDevice(d))
	}
	aero.Devices = out
	return out
}

func (aero *Aero) getStatus(d device.Device) device.Device {
	conn, c, ctx, cancel := aero.createClient(d)
	out := device.Device{}
	defer conn.Close()
	defer cancel()

	device, err := c.Status(ctx, &api.Void{})
	if err != nil {
		logger.Log("error", "error sending data: "+err.Error())
		return out
	}

	out = *aero.generateDeviceFromAPIDevice(device)
	return out
}

func (aero *Aero) fetchFile(d device.Device, fileIdx int) (bool, string) {
	conn, c, ctx, cancel := aero.createClient(d)
	defer conn.Close()
	defer cancel()

	if fileIdx < 0 || fileIdx >= len(d.Files) {
		return false, "file doesn't exists in the device"
	}

	resp, err := c.Fetch(ctx, &api.File{Hash: d.Files[fileIdx].Hash})
	if err != nil {
		logger.Log("error", "error sending data: "+err.Error())
		return false, "error sending file request"
	}

	return resp.Success, resp.Error
}

func (aero *Aero) createClient(d device.Device) (*grpc.ClientConn, api.ServiceClient, context.Context, context.CancelFunc) {
	var (
		conn *grpc.ClientConn
		err  error
	)

	conn, err = grpc.Dial(d.Ip+":"+d.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		logger.Log("error", "connection error: "+err.Error())
		os.Exit(1)
	}
	c := api.NewServiceClient(conn)
	token := aero.generateToken()
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{"jwt": token})), time.Second*10)
	return conn, c, ctx, cancel
}

func (aero *Aero) authInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Log("error", "cannot parse meta")
		return nil, status.Error(codes.Unauthenticated, "INTERNAL_SERVER_ERROR")
	}
	if len(meta["jwt"]) != 1 {
		logger.Log("error", "cannot parse meta - token empty")
		return nil, status.Error(codes.Unauthenticated, "token empty")
	}
	if !auth.ValidToken(meta["jwt"][0]) {
		logger.Log("error", "auth error")
		return nil, status.Error(codes.PermissionDenied, "invalid auth token")
	}
	return handler(ctx, req)
}

func (aero *Aero) generateToken() string {
	token, err := auth.GenerateJWT()
	if err != nil {
		logger.Log("error", "error generating token: "+err.Error())
		os.Exit(1)
	}
	return token
}

func (aero *Aero) generateAPIDeviceFromDevice(d *device.Device) *api.Device {
	files := make([]*api.File, 0)
	for _, f := range d.Files {
		files = append(files, &api.File{
			Name: f.Name,
			Hash: f.Hash,
			Ext:  f.Ext,
			Type: f.Type,
			Size: f.Size,
		})
	}
	return &api.Device{
		Hash:       d.Hash,
		Name:       d.Name,
		Ip:         d.Ip,
		Port:       d.Port,
		SocketPort: d.SocketPort,
		Active:     d.Active,
		Files:      files,
	}
}

func (aero *Aero) generateDeviceFromAPIDevice(d *api.Device) *device.Device {
	files := make([]file.File, 0)
	for _, f := range d.Files {
		files = append(files, *aero.generateFileFromAPIFile(f))
	}
	return &device.Device{
		Hash:       d.Hash,
		Name:       d.Name,
		Ip:         d.Ip,
		Port:       d.Port,
		SocketPort: d.SocketPort,
		Active:     d.Active,
		Files:      files,
	}
}

func (aero *Aero) generateFileFromAPIFile(f *api.File) *file.File {
	return &file.File{
		Name: f.Name,
		Hash: f.Hash,
		Ext:  f.Ext,
		Type: f.Type,
		Size: f.Size,
	}
}
