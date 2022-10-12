package device

import (
	"github.com/dhamith93/aero/file"
)

type Device struct {
	Hash       string      `json:"hash,omitempty"`
	Name       string      `json:"name,omitempty"`
	Ip         string      `json:"ip,omitempty"`
	Port       string      `json:"port,omitempty"`
	SocketPort string      `json:"socketPort,omitempty"`
	Files      []file.File `json:"files,omitempty"`
	Active     bool        `json:"active,omitempty"`
}
