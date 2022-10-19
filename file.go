package aero

import (
	"crypto/sha256"
	b64 "encoding/base64"
	"io"
	"os"

	"github.com/dhamith93/aero/internal/api"
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

func NewFile(path string) File {
	file := File{}
	file.Path = path
	file.setMeta()
	return file
}

func (file *File) setMeta() error {
	f, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	mtype, err := mimetype.DetectFile(file.Path)
	if err != nil {
		return err
	}

	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}

	file.Ext = mtype.Extension()
	file.Type = mtype.String()
	file.Size = fileInfo.Size()
	file.Name = fileInfo.Name()
	file.Hash, err = GetHash(f)
	return err
}

func GetHash(f *os.File) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return b64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func GenerateFileFromAPIFile(f *api.File) *File {
	return &File{
		Name: f.Name,
		Hash: f.Hash,
		Ext:  f.Ext,
		Type: f.Type,
		Size: f.Size,
	}
}
