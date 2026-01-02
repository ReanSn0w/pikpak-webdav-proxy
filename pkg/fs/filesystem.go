package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/utils"
	"github.com/go-pkgz/lgr"
	"golang.org/x/net/webdav"
)

type Webdav interface {
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.FileInfo, error)
	ReadStreamRange(path string, offset int64, length int64) (io.ReadCloser, error)
	MkdirAll(name string, perm os.FileMode) error
	RemoveAll(name string) error
	Rename(oldName, newName string, overwrite bool) error
}

type PikpakProxy struct {
	log          lgr.L
	localPath    string
	remoteClient Webdav
}

func NewPikpakProxy(log lgr.L, localPath string, remoteClient Webdav) *PikpakProxy {
	return &PikpakProxy{
		log:          log,
		localPath:    localPath,
		remoteClient: remoteClient,
	}
}

func (p *PikpakProxy) LocalFilePath(name string) string {
	name = strings.TrimPrefix(path.Clean(name), "/")
	return filepath.Join(p.localPath, name)
}

func (p *PikpakProxy) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	localPath := p.LocalFilePath(name)

	localErr := os.MkdirAll(localPath, perm)
	remoteErr := p.remoteClient.MkdirAll(name, perm)
	if localErr != nil && remoteErr != nil {
		return fmt.Errorf("failed to create local and remote directories: %w", localErr)
	}

	return nil
}

func (p *PikpakProxy) RemoveAll(ctx context.Context, name string) error {
	localPath := p.LocalFilePath(name)

	errLocal := os.RemoveAll(localPath)
	errRemote := p.remoteClient.RemoveAll(name)
	if errRemote != nil && errLocal != nil {
		return fmt.Errorf("failed to remove local and remote files: %w", errLocal)
	}

	return nil
}

func (p *PikpakProxy) Rename(ctx context.Context, oldName, newName string) error {
	oldPath := p.LocalFilePath(oldName)
	newPath := p.LocalFilePath(newName)

	localErr := os.Rename(oldPath, newPath)
	remoteErr := p.remoteClient.Rename(oldName, newName, true)
	if localErr != nil && remoteErr != nil {
		return fmt.Errorf("failed to rename local and remote files: %w", localErr)
	}

	return nil
}

func (p *PikpakProxy) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	localPath := p.LocalFilePath(name)

	if info, err := os.Stat(localPath); err == nil && name != "/" {
		p.log.Logf("[DEBUG] Stat (local): %s", name)
		return info, nil
	}

	p.log.Logf("[DEBUG] Stat (remote): %s", name)
	return p.remoteClient.Stat(name)
}

func (p *PikpakProxy) Readdir(ctx context.Context, name string) ([]os.FileInfo, error) {
	localPath := p.LocalFilePath(name)

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
	localPath := p.LocalFilePath(name)

	p.log.Logf("[DEBUG] OpenFile called for: %s (flag: %d)", name, flag)

	// Пытаемся открыть локально
	if f, err := os.OpenFile(localPath, flag, perm); err == nil {
		// Проверяем, является ли файл директорией
		if info, err := f.Stat(); err == nil && info.IsDir() {
			f.Close()
			p.log.Logf("[DEBUG] Opening directory as proxy: %s", name)
			return &utils.DirectoryFile{
				Name:     name,
				Proxy:    p,
				FileInfo: info,
			}, nil
		}

		p.log.Logf("[DEBUG] Opened local file: %s", name)
		return f, nil
	}

	// Для удаленного файла
	if flag&os.O_RDONLY != 0 || flag == 0 {
		p.log.Logf("[DEBUG] Opening remote file: %s", name)
		return utils.NewRemoteFile(p.remoteClient, name)
	}

	p.log.Logf("[ERROR] Cannot write to remote file: %s", name)
	return nil, os.ErrNotExist
}
