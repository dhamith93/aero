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

func (file *File) SetMeta() {
	mtype, err := mimetype.DetectFile(file.Path)
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}
	fi, err := os.Stat(file.Path)
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}

	file.Ext = mtype.Extension()
	file.Type = mtype.String()
	file.Size = fi.Size()
	file.Name = fi.Name()
	file.SetHash()
}

func (file *File) SetHash() {
	f, err := os.Open(file.Path)
	if err != nil {
		logger.Log("ERR", err.Error())
		return
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logger.Log("ERR", err.Error())
		return
	}
	file.Hash = b64.StdEncoding.EncodeToString(h.Sum(nil))
}
