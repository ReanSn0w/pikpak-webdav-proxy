package utils

import (
	"context"
	"io"
	"os"
)

type DirectoryProxy interface {
	Readdir(ctx context.Context, name string) ([]os.FileInfo, error)
}

type DirectoryFile struct {
	Name      string
	Proxy     DirectoryProxy
	FileInfo  os.FileInfo
	Files     []os.FileInfo
	FileIndex int
}

func (d *DirectoryFile) Close() error {
	return nil
}

func (d *DirectoryFile) Read(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}

func (d *DirectoryFile) Seek(offset int64, whence int) (int64, error) {
	return 0, os.ErrInvalid
}

func (d *DirectoryFile) Stat() (os.FileInfo, error) {
	return d.FileInfo, nil
}

func (d *DirectoryFile) Readdir(count int) ([]os.FileInfo, error) {
	// Если файлы еще не загружены
	if d.Files == nil {
		files, err := d.Proxy.Readdir(context.Background(), d.Name)
		if err != nil {
			return nil, err
		}
		d.Files = files
		d.FileIndex = 0
	}

	// Если count <= 0, возвращаем все
	if count <= 0 {
		result := d.Files[d.FileIndex:]
		d.FileIndex = len(d.Files)
		return result, nil
	}

	// Возвращаем count элементов
	end := d.FileIndex + count
	if end > len(d.Files) {
		end = len(d.Files)
	}

	result := d.Files[d.FileIndex:end]
	d.FileIndex = end

	if end >= len(d.Files) {
		return result, io.EOF
	}

	return result, nil
}

func (d *DirectoryFile) Write(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}
