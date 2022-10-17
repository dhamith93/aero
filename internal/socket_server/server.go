package socketserver

import (
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/dhamith93/aero/device"
	"github.com/dhamith93/aero/file"
	"github.com/dhamith93/aero/internal/api"
	"github.com/dhamith93/aero/internal/logger"
)

type SocketServer struct {
	Port    string
	Devices *[]*api.Device
	Self    *device.Device
}

func (s *SocketServer) Start() {
	server, err := net.Listen("tcp", "0.0.0.0"+":"+s.Port)
	if err != nil {
		logger.Log("ERR", "socket_server start: "+err.Error())
		os.Exit(1)
	}
	defer server.Close()
	logger.Log("MSG", "socket_server: listening on 0.0.0.0"+":"+s.Port)
	for {
		connection, err := server.Accept()
		if err != nil {
			logger.Log("ERR", "socket_server accepting client: "+err.Error())
			os.Exit(1)
		}
		go s.processClient(connection)
	}
}

func (s *SocketServer) processClient(connection net.Conn) {
	defer connection.Close()
	logger.Log("MSG", "send_file: serving client: "+connection.RemoteAddr().String())
	remoteAddr := strings.Split(connection.RemoteAddr().String(), ":")

	if len(remoteAddr) < 2 {
		logger.Log("ERR", "send_file: cannot parse remote address to verification")
		return
	}

	found := false
	for _, device := range *s.Devices {
		if device.Ip == remoteAddr[0] {
			found = true
		}
	}

	if !found {
		logger.Log("ERR", "send_file: incoming device not found in list "+remoteAddr[0])
		return
	}

	found = false
	buffer := make([]byte, 1024)
	mLen, err := connection.Read(buffer)
	if err != nil {
		logger.Log("ERR", "send_file: "+err.Error())
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
		logger.Log("ERR", "send_file: requested file not found in list "+requestedFileHash)
		return
	}

	file, err := os.Open(strings.TrimSpace(outputFile.Path))
	if err != nil {
		logger.Log("ERR", "send_file: "+err.Error())
		return
	}
	defer file.Close()

	logger.Log("MSG", "send_file: sending "+outputFile.Name)
	_, err = io.Copy(connection, file)
	if err != nil {
		logger.Log("ERR", "send_file: "+err.Error())
	}
}

func (s *SocketServer) RequestFile(d device.Device, fileIdx int) {
	connection, err := net.Dial("tcp", d.Ip+":"+d.SocketPort)
	if err != nil {
		logger.Log("ERR", "request_file: "+err.Error())
		return
	}
	defer connection.Close()

	_, err = connection.Write([]byte(d.Files[fileIdx].Hash))
	if err != nil {
		logger.Log("ERR", "request_file: "+err.Error())
		return
	}

	newFile, err := os.Create(d.Files[fileIdx].Name)
	if err != nil {
		logger.Log("ERR", "request_file: "+err.Error())
		return
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, connection)
	if err != nil {
		log.Fatal(err)
	}

	createdFile := file.New(d.Files[fileIdx].Name)
	if d.Files[fileIdx].Hash == createdFile.Hash {
		logger.Log("MSG", "rec: file: "+createdFile.Name+" from: "+d.Name+" "+d.Ip)
	} else {
		logger.Log("ERR", "request_file: file transfer failed, hash mismatch")
	}
}
