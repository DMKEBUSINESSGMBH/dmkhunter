package model

import (
	"crypto/md5"
	"io"
	"os"
	"sync"
)

type File struct {
	Path string
	File os.File
}

// hashBufferSize is the read buffer per Hash call, larger than io.Copy's 32KB default.
const hashBufferSize = 256 * 1024

var hashBuffers = sync.Pool{
	New: func() any {
		b := make([]byte, hashBufferSize)
		return &b
	},
}

// NewFile does not touch the filesystem, the file is only opened when it is hashed.
func NewFile(p string) *File {
	return &File{
		Path: p,
	}
}

func (file File) Hash() ([]byte, error) {
	f, err := os.Open(file.Path)

	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := hashBuffers.Get().(*[]byte)
	defer hashBuffers.Put(buf)

	h := md5.New()

	// wrap f so io.CopyBuffer can't bypass buf via (*os.File).WriteTo
	if _, err := io.CopyBuffer(h, struct{ io.Reader }{f}, *buf); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func (file File) Info() (*os.FileInfo, error) {
	s, err := os.Stat(file.Path)

	if err != nil {
		return nil, err
	}

	return &s, err
}

func (file File) IsSymlink() (bool, error) {
	fileInfo, err := file.Info()

	if err != nil {
		return false, err
	}

	return (*fileInfo).Mode()&os.ModeSymlink == os.ModeSymlink, nil
}
