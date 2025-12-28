package fs

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/studio-b12/gowebdav"
	"golang.org/x/net/webdav"
)

type remoteFile struct {
	filepath       string
	client         *gowebdav.Client
	localCachePath string
	offset         int64
	size           int64
	isDir          bool
	fileInfo       os.FileInfo
	localFile      *os.File
	isDirty        bool
	reader         io.ReadCloser

	// Защита от race conditions при параллельных читаниях
	mu sync.Mutex
}

func newRemoteFile(client *gowebdav.Client, filepath string, localCachePath string) (webdav.File, error) {
	stat, err := client.Stat(filepath)
	if err != nil {
		log.Printf("[ERROR] Failed to stat remote file %s: %v", filepath, err)
		return nil, err
	}

	log.Printf("[DEBUG] Opened remote file: %s (size: %d, isDir: %v)", filepath, stat.Size(), stat.IsDir())

	return &remoteFile{
		filepath:       filepath,
		client:         client,
		localCachePath: localCachePath,
		offset:         0,
		size:           stat.Size(),
		isDir:          stat.IsDir(),
		fileInfo:       stat,
		isDirty:        false,
		reader:         nil,
	}, nil
}

// Read читает данные с поддержкой Range запросов
func (f *remoteFile) Read(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isDir {
		return 0, os.ErrInvalid
	}

	// Если достигли конца файла
	if f.offset >= f.size {
		log.Printf("[DEBUG] remoteFile.Read: EOF (offset=%d, size=%d)", f.offset, f.size)
		return 0, io.EOF
	}

	toRead := int64(len(p))
	remaining := f.size - f.offset
	if toRead > remaining {
		toRead = remaining
	}

	log.Printf("[DEBUG] remoteFile.Read (Range): reading %d bytes at offset %d (goroutine-safe)", toRead, f.offset)

	reader, err := f.client.ReadStreamRange(f.filepath, f.offset, toRead)
	if err != nil {
		log.Printf("[ERROR] remoteFile.Read: ReadStreamRange failed: %v", err)
		return 0, err
	}
	defer reader.Close()

	readLen := int(toRead)
	if readLen > len(p) {
		readLen = len(p)
	}

	n, err = reader.Read(p[:readLen])
	if n > 0 {
		f.offset += int64(n)
		log.Printf("[DEBUG] remoteFile.Read (Range): read %d bytes, offset now %d/%d", n, f.offset, f.size)
	}

	return n, err
}

// Seek изменяет позицию в файле
func (f *remoteFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isDir {
		return 0, os.ErrInvalid
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		newOffset = f.size + offset
	default:
		return 0, os.ErrInvalid
	}

	if newOffset < 0 {
		return 0, os.ErrInvalid
	}

	if newOffset > f.size {
		newOffset = f.size
	}

	// Закрываем старый reader если был
	if f.reader != nil {
		f.reader.Close()
		f.reader = nil
	}

	log.Printf("[DEBUG] remoteFile.Seek: from %d to %d (size: %d)", f.offset, newOffset, f.size)
	f.offset = newOffset
	return f.offset, nil
}

// Write пишет данные в локальный кеш файла
func (f *remoteFile) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isDir {
		return 0, os.ErrInvalid
	}

	if f.localFile == nil {
		os.MkdirAll(filepath.Dir(f.localCachePath), 0755)
		localFile, err := os.OpenFile(f.localCachePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		f.localFile = localFile
	}

	f.isDirty = true
	return f.localFile.Write(p)
}

// Close закрывает файл
func (f *remoteFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errs []error

	if f.reader != nil {
		if err := f.reader.Close(); err != nil {
			log.Printf("[ERROR] Failed to close remote stream: %v", err)
			errs = append(errs, err)
		}
		f.reader = nil
	}

	if f.localFile != nil {
		if err := f.localFile.Close(); err != nil {
			log.Printf("[ERROR] Failed to close local file: %v", err)
			errs = append(errs, err)
		}
		f.localFile = nil
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// Stat возвращает информацию о файле
func (f *remoteFile) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fileInfo == nil {
		stat, err := f.client.Stat(f.filepath)
		if err != nil {
			return nil, err
		}
		f.fileInfo = stat
	}
	return f.fileInfo, nil
}

// Readdir читает список файлов директории
func (f *remoteFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.isDir {
		return nil, os.ErrInvalid
	}

	files, err := f.client.ReadDir(f.filepath)
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
