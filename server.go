package aero

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

type ProgressWriter struct {
	FileSize    int64
	Received    int64
	Progress    int
	HashMatched bool
	Error       error
}

func (pw *ProgressWriter) Write(data []byte) (int, error) {
	pw.Received += int64(len(data))
	pw.Progress = int((pw.Received * 100) / pw.FileSize)
	return pw.Progress, nil
}

type SocketServer struct {
	Port      string
	Devices   *[]Device
	Self      *Device
	server    net.Listener
	Messages  Messages
	Downloads map[int]*ProgressWriter
}

func (s *SocketServer) Start() error {
	var err error
	s.server, err = net.Listen("tcp", "0.0.0.0"+":"+s.Port)
	if err != nil {
		return err
	}
	defer s.server.Close()
	s.Downloads = make(map[int]*ProgressWriter)
	for {
		connection, err := s.server.Accept()
		if err != nil {
			return err
		}
		go s.handleFileRequest(connection)
	}
}

func (s *SocketServer) Stop() {
	s.server.Close()
}

func (s *SocketServer) handleFileRequest(connection net.Conn) {
	defer connection.Close()
	s.Messages.Add("send_file: serving client: "+connection.RemoteAddr().String(), MSG)
	remoteAddr := strings.Split(connection.RemoteAddr().String(), ":")

	if len(remoteAddr) < 2 {
		s.Messages.Add("send_file: cannot parse remote address to verification", ERR)
		return
	}

	found := false
	for _, device := range *s.Devices {
		if device.Ip == remoteAddr[0] {
			found = true
		}
	}

	if !found {
		s.Messages.Add("send_file: incoming device not found in list "+remoteAddr[0], ERR)
		return
	}

	found = false
	buffer := make([]byte, 1024)
	mLen, err := connection.Read(buffer)
	if err != nil {
		s.Messages.Add("send_file: "+err.Error(), ERR)
		return
	}

	requestedFileHash := string(buffer[:mLen])
	outputFile := File{}

	for _, file := range s.Self.Files {
		if file.Hash == requestedFileHash {
			found = true
			outputFile = file
		}
	}

	if !found {
		s.Messages.Add("send_file: requested file not found in list "+requestedFileHash, ERR)
		return
	}

	file, err := os.Open(strings.TrimSpace(outputFile.Path))
	if err != nil {
		s.Messages.Add("send_file: "+err.Error(), ERR)
		return
	}
	defer file.Close()

	s.Messages.Add("send_file: sending "+outputFile.Name, MSG)
	_, err = io.Copy(connection, file)
	if err != nil {
		s.Messages.Add("send_file: "+err.Error(), ERR)
	}
}

func (s *SocketServer) Download(d Device, fileIdx int) int {
	if s.Downloads == nil {
		s.Downloads = make(map[int]*ProgressWriter)
	}

	id := len(s.Downloads) + 1
	s.Downloads[id] = &ProgressWriter{FileSize: d.Files[fileIdx].Size}
	go s.download(d, fileIdx, id)
	return id
}

func (s *SocketServer) download(d Device, fileIdx int, downloadId int) {
	progressWriter := s.Downloads[downloadId]
	connection, err := net.Dial("tcp", d.Ip+":"+d.SocketPort)
	if err != nil {
		progressWriter.Error = err
		return
	}
	defer connection.Close()

	_, err = connection.Write([]byte(d.Files[fileIdx].Hash))
	if err != nil {
		progressWriter.Error = err
		return
	}

	newFile, err := os.Create(d.Files[fileIdx].Name)
	if err != nil {
		progressWriter.Error = err
		return
	}
	defer newFile.Close()

	rdr := io.TeeReader(connection, progressWriter)

	_, err = io.Copy(newFile, rdr)
	if err != nil {
		progressWriter.Error = err
		return
	}

	createdFile := NewFile(d.Files[fileIdx].Name)
	if d.Files[fileIdx].Hash != createdFile.Hash {
		err := fmt.Errorf("file transfer failed due to hash mismatch. want %s have %s", d.Files[fileIdx].Hash, createdFile.Hash)
		progressWriter.Error = err
		progressWriter.HashMatched = false
		return
	}

	s.Messages.Add("received file: "+createdFile.Name+" from: "+d.Name+" "+d.Ip, MSG)
	progressWriter.HashMatched = true
}
