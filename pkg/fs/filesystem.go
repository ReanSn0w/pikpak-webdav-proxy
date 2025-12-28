package fs

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-pkgz/lgr"
	"github.com/studio-b12/gowebdav"
	"golang.org/x/net/webdav"
)

type PikpakProxy struct {
	log          lgr.L
	localPath    string
	remoteClient *gowebdav.Client
}

func NewPikpakProxy(log lgr.L, localPath string, remoteClient *gowebdav.Client) webdav.FileSystem {
	return &PikpakProxy{
		log:          log,
		localPath:    localPath,
		remoteClient: remoteClient,
	}
}

func (p *PikpakProxy) localFilePath(name string) string {
	name = strings.TrimPrefix(path.Clean(name), "/")
	return filepath.Join(p.localPath, name)
}

func (p *PikpakProxy) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	localPath := p.localFilePath(name)
	return os.MkdirAll(localPath, perm)
}

func (p *PikpakProxy) RemoveAll(ctx context.Context, name string) error {
	localPath := p.localFilePath(name)
	return os.RemoveAll(localPath)
}

func (p *PikpakProxy) Rename(ctx context.Context, oldName, newName string) error {
	oldPath := p.localFilePath(oldName)
	newPath := p.localFilePath(newName)

	return os.Rename(oldPath, newPath)
}

func (p *PikpakProxy) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	localPath := p.localFilePath(name)

	if info, err := os.Stat(localPath); err == nil && name != "/" {
		p.log.Logf("[DEBUG] Stat (local): %s", name)
		return info, nil
	}

	p.log.Logf("[DEBUG] Stat (remote): %s", name)
	return p.remoteClient.Stat(name)
}

func (p *PikpakProxy) Readdir(ctx context.Context, name string) ([]os.FileInfo, error) {
	localPath := p.localFilePath(name)

	p.log.Logf("[DEBUG] Readdir called for: %s (local: %s)", name, localPath)

	localFiles, err := os.ReadDir(localPath)
	if err != nil && !os.IsNotExist(err) {
		p.log.Logf("[ERROR] ReadDir local error: %v", err)
		return nil, err
	}

	var result []os.FileInfo
	localNames := make(map[string]bool)

	for _, f := range localFiles {
		info, err := f.Info()
		if err != nil {
			continue
		}
		result = append(result, info)
		localNames[f.Name()] = true
		p.log.Logf("[DEBUG] Local file: %s", f.Name())
	}

	p.log.Logf("[DEBUG] Fetching remote files for: %s", name)
	remoteFiles, err := p.remoteClient.ReadDir(name)
	if err != nil {
		p.log.Logf("[WARN] Failed to read remote dir: %v", err)
	} else {
		p.log.Logf("[DEBUG] Found %d remote files", len(remoteFiles))
		for _, f := range remoteFiles {
			if !localNames[f.Name()] {
				result = append(result, f)
				p.log.Logf("[DEBUG] Adding from remote: %s", f.Name())
			}
		}
	}

	p.log.Logf("[DEBUG] Readdir result: %d total files", len(result))
	return result, nil
}

func (p *PikpakProxy) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	localPath := p.localFilePath(name)

	p.log.Logf("[DEBUG] OpenFile called for: %s (flag: %d)", name, flag)

	// Пытаемся открыть локально
	if f, err := os.OpenFile(localPath, flag, perm); err == nil {
		// Проверяем, является ли файл директорией
		if info, err := f.Stat(); err == nil && info.IsDir() {
			f.Close()
			p.log.Logf("[DEBUG] Opening directory as proxy: %s", name)
			return &directoryFile{
				name:     name,
				proxy:    p,
				fileInfo: info,
			}, nil
		}

		p.log.Logf("[DEBUG] Opened local file: %s", name)
		return f, nil
	}

	// Для удаленного файла
	if flag&os.O_RDONLY != 0 || flag == 0 {
		p.log.Logf("[DEBUG] Opening remote file: %s", name)
		return newRemoteFile(p.remoteClient, name, localPath)
	}

	p.log.Logf("[ERROR] Cannot write to remote file: %s", name)
	return nil, os.ErrNotExist
}
