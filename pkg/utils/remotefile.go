package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"golang.org/x/net/webdav"
)

type WebDav interface {
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.FileInfo, error)
	ReadStreamRange(path string, offset int64, length int64) (io.ReadCloser, error)
}

type remoteFile struct {
	Filepath       string
	Client         WebDav
	LocalCachePath string
	Offset         int64
	Size           int64
	IsDir          bool
	FileInfo       os.FileInfo

	// Защита от race conditions при параллельных читаниях
	mu sync.RWMutex
}

func NewRemoteFile(client WebDav, filepath string) (webdav.File, error) {
	stat, err := client.Stat(filepath)
	if err != nil {
		log.Printf("[ERROR] Failed to stat remote file %s: %v", filepath, err)
		return nil, err
	}

	log.Printf("[DEBUG] Opened remote file: %s (size: %d, isDir: %v)", filepath, stat.Size(), stat.IsDir())

	return &remoteFile{
		Filepath: filepath,
		Client:   client,
		Offset:   0,
		Size:     stat.Size(),
		IsDir:    stat.IsDir(),
		FileInfo: stat,
	}, nil
}

// Read читает данные с поддержкой Range запросов
func (f *remoteFile) Read(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.IsDir {
		return 0, os.ErrInvalid
	}

	// Если достигли конца файла
	if f.Offset >= f.Size {
		log.Printf("[DEBUG] remoteFile.Read: EOF (offset=%d, size=%d)", f.Offset, f.Size)
		return 0, io.EOF
	}

	toRead := min(int64(len(p)), f.Size-f.Offset)

	log.Printf("[DEBUG] remoteFile.Read (Range): reading %d bytes at offset %d (goroutine-safe)", toRead, f.Offset)

	reader, err := f.Client.ReadStreamRange(f.Filepath, f.Offset, toRead)
	if err != nil {
		log.Printf("[ERROR] remoteFile.Read: ReadStreamRange failed: %v", err)
		return 0, err
	}
	defer reader.Close()

	readLen := min(int(toRead), len(p))

	n, err = reader.Read(p[:readLen])
	if n > 0 {
		f.Offset += int64(n)
		log.Printf("[DEBUG] remoteFile.Read (Range): read %d bytes, offset now %d/%d", n, f.Offset, f.Size)
	}

	return n, err
}

// Seek изменяет позицию в файле
func (f *remoteFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.IsDir {
		return 0, os.ErrInvalid
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.Offset + offset
	case io.SeekEnd:
		newOffset = f.Size + offset
	default:
		return 0, os.ErrInvalid
	}

	if newOffset < 0 {
		return 0, os.ErrInvalid
	}

	if newOffset > f.Size {
		newOffset = f.Size
	}

	log.Printf("[DEBUG] remoteFile.Seek: from %d to %d (size: %d)", f.Offset, newOffset, f.Size)
	f.Offset = newOffset
	return f.Offset, nil
}

// Write пишет данные в локальный кеш файла
func (f *remoteFile) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write to remote file not enabled")
}

// Close закрывает файл
func (f *remoteFile) Close() error {
	return nil
}

// Stat возвращает информацию о файле
func (f *remoteFile) Stat() (os.FileInfo, error) {
	return f.FileInfo, nil
}

// Readdir читает список файлов директории
func (f *remoteFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.IsDir {
		return nil, os.ErrInvalid
	}

	files, err := f.Client.ReadDir(f.Filepath)
	if err != nil {
		return nil, err
	}

	if count <= 0 {
		return files, nil
	}

	if len(files) > count {
		return files[:count], nil
	}

	return files, nil
}
