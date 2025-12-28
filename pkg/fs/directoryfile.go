package fs

import (
	"context"
	"io"
	"os"
)

type directoryFile struct {
	name      string
	proxy     *PikpakProxy
	fileInfo  os.FileInfo
	files     []os.FileInfo
	fileIndex int
}

func (d *directoryFile) Close() error {
	return nil
}

func (d *directoryFile) Read(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}

func (d *directoryFile) Seek(offset int64, whence int) (int64, error) {
	return 0, os.ErrInvalid
}

func (d *directoryFile) Stat() (os.FileInfo, error) {
	return d.fileInfo, nil
}

func (d *directoryFile) Readdir(count int) ([]os.FileInfo, error) {
	// Если файлы еще не загружены
	if d.files == nil {
		files, err := d.proxy.Readdir(context.Background(), d.name)
		if err != nil {
			return nil, err
		}
		d.files = files
		d.fileIndex = 0
	}

	// Если count <= 0, возвращаем все
	if count <= 0 {
		result := d.files[d.fileIndex:]
		d.fileIndex = len(d.files)
		return result, nil
	}

	// Возвращаем count элементов
	end := d.fileIndex + count
	if end > len(d.files) {
		end = len(d.files)
	}

	result := d.files[d.fileIndex:end]
	d.fileIndex = end

	if end >= len(d.files) {
		return result, io.EOF
	}

	return result, nil
}

func (d *directoryFile) Write(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}
