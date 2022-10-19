package socketserver

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/dhamith93/aero/device"
	"github.com/dhamith93/aero/file"
	"github.com/dhamith93/aero/internal/api"
)

type SocketServer struct {
	Port    string
	Devices *[]*api.Device
	Self    *device.Device
	server  net.Listener
}

func (s *SocketServer) Start() error {
	var err error
	s.server, err = net.Listen("tcp", "0.0.0.0"+":"+s.Port)
	if err != nil {
		return err
	}
	defer s.server.Close()
	for {
		connection, err := s.server.Accept()
		if err != nil {
			return err
		}
		go s.processClient(connection)
	}
}

func (s *SocketServer) Stop() {
	s.server.Close()
}

func (s *SocketServer) processClient(connection net.Conn) {
	defer connection.Close()
	// logger.Log("MSG", "send_file: serving client: "+connection.RemoteAddr().String())
	remoteAddr := strings.Split(connection.RemoteAddr().String(), ":")

	if len(remoteAddr) < 2 {
		// logger.Log("ERR", "send_file: cannot parse remote address to verification")
		return
	}

	found := false
	for _, device := range *s.Devices {
		if device.Ip == remoteAddr[0] {
			found = true
		}
	}

	if !found {
		// logger.Log("ERR", "send_file: incoming device not found in list "+remoteAddr[0])
		return
	}

	found = false
	buffer := make([]byte, 1024)
	mLen, err := connection.Read(buffer)
	if err != nil {
		// logger.Log("ERR", "send_file: "+err.Error())
		return
	}

	requestedFileHash := string(buffer[:mLen])
	outputFile := file.File{}

	for _, file := range s.Self.Files {
		if file.Hash == requestedFileHash {
			found = true
			outputFile = file
		}
	}

	if !found {
		// logger.Log("ERR", "send_file: requested file not found in list "+requestedFileHash)
		return
	}

	file, err := os.Open(strings.TrimSpace(outputFile.Path))
	if err != nil {
		// logger.Log("ERR", "send_file: "+err.Error())
		return
	}
	defer file.Close()

	// logger.Log("MSG", "send_file: sending "+outputFile.Name)
	_, err = io.Copy(connection, file)
	if err != nil {
		// logger.Log("ERR", "send_file: "+err.Error())
	}
}

func (s *SocketServer) RequestFile(d device.Device, fileIdx int) error {
	connection, err := net.Dial("tcp", d.Ip+":"+d.SocketPort)
	if err != nil {
		return err
	}
	defer connection.Close()

	_, err = connection.Write([]byte(d.Files[fileIdx].Hash))
	if err != nil {
		return err
	}

	newFile, err := os.Create(d.Files[fileIdx].Name)
	if err != nil {
		return err
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, connection)
	if err != nil {
		return err
	}

	createdFile := file.New(d.Files[fileIdx].Name)
	if d.Files[fileIdx].Hash != createdFile.Hash {
		return fmt.Errorf("file transfer failed due to hash mismatch. want %s have %s", d.Files[fileIdx].Hash, createdFile.Hash)
	}

	// logger.Log("MSG", "rec: file: "+createdFile.Name+" from: "+d.Name+" "+d.Ip)
	return nil
}
