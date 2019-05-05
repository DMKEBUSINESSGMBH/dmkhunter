package model

import (
	"crypto/md5"
	"io"
	"os"
)

type File struct {
	Path string
	File os.File
}

func NewFile(p string) (*File, error) {
	f, err := os.Open(p)
	defer f.Close()

	if err != nil {
		return nil, err
	}

	return &File{
		Path: p,
	}, nil
}


func (file File) Hash() ([]byte, error) {
	f, err := os.Open(file.Path)
	defer f.Close()

	if err != nil {
		return nil, err
	}

	h := md5.New()

	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func (f File) Info() (*os.FileInfo, error) {
	s, err := os.Stat(f.Path)

	if err != nil {
		return nil, err
	}

	return &s, err
}
