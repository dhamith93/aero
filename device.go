package aero

import (
	"github.com/dhamith93/aero/internal/api"
)

type Device struct {
	Name       string `json:"name,omitempty"`
	Ip         string `json:"ip,omitempty"`
	Port       string `json:"port,omitempty"`
	SocketPort string `json:"socketPort,omitempty"`
	Files      []File `json:"files,omitempty"`
}

func GenerateAPIDeviceFromDevice(d *Device) *api.Device {
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
		Name:       d.Name,
		Ip:         d.Ip,
		Port:       d.Port,
		SocketPort: d.SocketPort,
		Files:      files,
	}
}

func GenerateDeviceFromAPIDevice(d *api.Device) *Device {
	files := make([]File, 0)
	for _, f := range d.Files {
		files = append(files, *GenerateFileFromAPIFile(f))
	}
	return &Device{
		Name:       d.Name,
		Ip:         d.Ip,
		Port:       d.Port,
		SocketPort: d.SocketPort,
		Files:      files,
	}
}
