package file

import (
	"crypto/sha256"
	b64 "encoding/base64"
	"io"
	"os"

	"github.com/dhamith93/aero/internal/logger"
	"github.com/gabriel-vasile/mimetype"
)

type File struct {
	Name string `json:"name,omitempty"`
	Hash string `json:"hash,omitempty"`
	Type string `json:"type,omitempty"`
	Ext  string `json:"ext,omitempty"`
	Path string `json:"path,omitempty"`
	Size int64  `json:"size,omitempty"`
}

func New(path string) File {
	file := File{}
	file.Path = path
	file.setMeta()
	return file
}

func (file *File) setMeta() {
	f, err := os.Open(file.Path)
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}
	defer f.Close()

	mtype, err := mimetype.DetectFile(file.Path)
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}

	fileInfo, err := f.Stat()
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}

	file.Ext = mtype.Extension()
	file.Type = mtype.String()
	file.Size = fileInfo.Size()
	file.Name = fileInfo.Name()
	file.Hash = GetHash(f)
}

func GetHash(f *os.File) string {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logger.Log("ERR", err.Error())
		return ""
	}
	return b64.StdEncoding.EncodeToString(h.Sum(nil))
}
