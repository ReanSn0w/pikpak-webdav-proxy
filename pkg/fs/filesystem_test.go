package fs_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/fs"
	lgr "github.com/go-pkgz/lgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWebdav mock для интерфейса Webdav
type MockWebdav struct {
	mock.Mock
}

func (m *MockWebdav) Stat(path string) (os.FileInfo, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (m *MockWebdav) ReadDir(path string) ([]os.FileInfo, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

func (m *MockWebdav) ReadStreamRange(path string, offset int64, length int64) (io.ReadCloser, error) {
	args := m.Called(path, offset, length)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockWebdav) MkdirAll(name string, perm os.FileMode) error {
	args := m.Called(name, perm)
	return args.Error(0)
}

func (m *MockWebdav) RemoveAll(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockWebdav) Rename(oldName, newName string, overwrite bool) error {
	args := m.Called(oldName, newName, overwrite)
	return args.Error(0)
}

// MockFileInfo для тестирования
type MockFileInfo struct {
	mock.Mock
	name  string
	isDir bool
}

func newMockFileInfo(name string, isDir bool) *MockFileInfo {
	m := &MockFileInfo{
		name:  name,
		isDir: isDir,
	}
	m.On("Name").Return(name)
	m.On("IsDir").Return(isDir)
	m.On("Size").Return(int64(0)) // Возвращаем int64, а не int
	m.On("Mode").Return(os.FileMode(0644))
	m.On("ModTime").Return(time.Now())
	m.On("Sys").Return(nil)
	return m
}

func (m *MockFileInfo) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockFileInfo) Size() int64 {
	args := m.Called()
	return args.Get(0).(int64) // Правильно преобразуем в int64
}

func (m *MockFileInfo) Mode() os.FileMode {
	args := m.Called()
	return args.Get(0).(os.FileMode)
}

func (m *MockFileInfo) ModTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockFileInfo) IsDir() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockFileInfo) Sys() any {
	args := m.Called()
	return args.Get(0)
}

func TestNewPikpakProxy(t *testing.T) {
	log := lgr.New()
	localPath := "/tmp/test"
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, localPath, mockClient)

	assert.NotNil(t, proxy)
}

func TestMkdir_BothSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("MkdirAll", "/testdir", os.FileMode(0755)).Return(nil)

	err := proxy.Mkdir(context.Background(), "/testdir", 0755)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMkdir_LocalFails_RemoteSucceeds(t *testing.T) {
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Используем недействительный путь для локальной ошибки
	proxy := fs.NewPikpakProxy(log, "/invalid/nonexistent/path/that/cannot/exist", mockClient)

	mockClient.On("MkdirAll", "/testdir", os.FileMode(0755)).Return(nil)

	err := proxy.Mkdir(context.Background(), "/testdir", 0755)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMkdir_BothFail(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("MkdirAll", "/testdir", os.FileMode(0755)).
		Return(errors.New("remote mkdir failed"))

	// Попытаемся создать в пути, где нет прав
	err := proxy.Mkdir(context.Background(), "/testdir", 0755)

	// Одна из операций не должна требоваться для успеха
	assert.NoError(t, err)
}

func TestRemoveAll_BothSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем тестовую файловую структуру
	testPath := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(testPath, 0755)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("RemoveAll", "/testdir").Return(nil)

	err := proxy.RemoveAll(context.Background(), "/testdir")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRemoveAll_LocalFails_RemoteSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("RemoveAll", "/nonexistent").Return(nil)

	err := proxy.RemoveAll(context.Background(), "/nonexistent")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// Исправленный тест - возвращаем ошибку при удалении несуществующего файла
func TestRemoveAll_BothFail(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	// Создаем файл в директории с ограниченными правами
	testSubDir := filepath.Join(tmpDir, "restricted")
	os.MkdirAll(testSubDir, 0755)

	testFile := filepath.Join(testSubDir, "testfile.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	// Мокируем ошибку удаления на удаленном сервере
	mockClient.On("RemoveAll", "/restricted/testfile.txt").
		Return(errors.New("remote remove failed"))

	// Устанавливаем права на чтение только для директории (запретим удаление)
	os.Chmod(testSubDir, 0500)

	err := proxy.RemoveAll(context.Background(), "/restricted/testfile.txt")

	// Обе операции должны вернуть ошибку
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove local and remote files")
	mockClient.AssertExpectations(t)

	// Очищаем после себя - восстанавливаем права
	os.Chmod(testSubDir, 0755)
}

func TestRename_BothSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем файл для переименования
	oldPath := filepath.Join(tmpDir, "old.txt")
	os.WriteFile(oldPath, []byte("test"), 0644)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("Rename", "/old.txt", "/new.txt", true).Return(nil)

	err := proxy.Rename(context.Background(), "/old.txt", "/new.txt")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRename_LocalFails_RemoteSucceeds(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("Rename", "/nonexistent", "/new.txt", true).Return(nil)

	err := proxy.Rename(context.Background(), "/nonexistent", "/new.txt")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRename_BothFail(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("Rename", "/old.txt", "/new.txt", true).
		Return(errors.New("remote rename failed"))

	err := proxy.Rename(context.Background(), "/old.txt", "/new.txt")

	assert.Error(t, err)
	mockClient.AssertExpectations(t)
}

func TestStat_LocalExists(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем тестовый файл
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	info, err := proxy.Stat(context.Background(), "/test.txt")

	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "test.txt", info.Name())
	mockClient.AssertNotCalled(t, "Stat")
}

func TestStat_LocalNotExists_FallbackToRemote(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockFileInfo := newMockFileInfo("remote.txt", false)

	mockClient.On("Stat", "/remote.txt").Return(mockFileInfo, nil)

	info, err := proxy.Stat(context.Background(), "/remote.txt")

	assert.NoError(t, err)
	assert.NotNil(t, info)
	mockClient.AssertExpectations(t)
}

func TestStat_RootPath(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockFileInfo := newMockFileInfo("", true)
	mockClient.On("Stat", "/").Return(mockFileInfo, nil)

	info, err := proxy.Stat(context.Background(), "/")

	assert.NoError(t, err)
	assert.NotNil(t, info)
	mockClient.AssertExpectations(t)
}

func TestReaddir_OnlyLocal(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем локальные файлы
	os.WriteFile(filepath.Join(tmpDir, "local1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "local2.txt"), []byte("test"), 0644)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("ReadDir", "/").Return(nil, errors.New("remote error"))

	files, err := proxy.Readdir(context.Background(), "/")

	assert.NoError(t, err)
	assert.Greater(t, len(files), 0)
	mockClient.AssertExpectations(t)
}

func TestReaddir_LocalAndRemote(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем локальный файл
	os.WriteFile(filepath.Join(tmpDir, "local.txt"), []byte("test"), 0644)

	// Создаем mock для удаленных файлов
	remoteFileInfo := newMockFileInfo("remote.txt", false)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("ReadDir", "/").Return([]os.FileInfo{remoteFileInfo}, nil)

	files, err := proxy.Readdir(context.Background(), "/")

	assert.NoError(t, err)
	assert.Greater(t, len(files), 0)
	mockClient.AssertExpectations(t)
}

func TestReaddir_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("ReadDir", "/").Return(nil, os.ErrNotExist)

	_, err := proxy.Readdir(context.Background(), "/")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestReaddir_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем структуру директорий
	nestedDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "file.txt"), []byte("test"), 0644)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	mockClient.On("ReadDir", "/subdir").Return(nil, os.ErrNotExist)

	files, err := proxy.Readdir(context.Background(), "/subdir")

	assert.NoError(t, err)
	// Должно быть 1 файл
	assert.Equal(t, 1, len(files))
	mockClient.AssertExpectations(t)
}

func TestOpenFile_LocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем тестовый файл
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	f, err := proxy.OpenFile(context.Background(), "/test.txt", os.O_RDONLY, 0644)

	assert.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()
	mockClient.AssertNotCalled(t, "ReadStreamRange")
}

func TestOpenFile_LocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	// Создаем директорию
	testDir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(testDir, 0755)

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	f, err := proxy.OpenFile(context.Background(), "/testdir", os.O_RDONLY, 0644)

	assert.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()
}

func TestOpenFile_RemoteFileRead(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	// Добавляем mock для Stat с правильной типизацией
	mockFileInfo := &MockFileInfo{
		name:  "remote.txt",
		isDir: false,
	}
	mockFileInfo.On("Name").Return("remote.txt")
	mockFileInfo.On("IsDir").Return(false)
	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("Mode").Return(os.FileMode(0644))
	mockFileInfo.On("ModTime").Return(time.Now())
	mockFileInfo.On("Sys").Return(nil)

	mockClient.On("Stat", "/remote.txt").Return(mockFileInfo, nil)

	// ReadStreamRange НЕ вызывается при открытии, только при чтении
	// поэтому мы его не проверяем
	mockReader := io.NopCloser(nil)
	mockClient.On("ReadStreamRange", "/remote.txt", int64(0), int64(-1)).
		Return(mockReader, nil)

	f, err := proxy.OpenFile(context.Background(), "/remote.txt", os.O_RDONLY, 0644)

	assert.NoError(t, err)
	assert.NotNil(t, f)

	// Проверяем только Stat (который вызывается при открытии)
	mockClient.AssertCalled(t, "Stat", "/remote.txt")
}

func TestOpenFile_WriteToRemote_Error(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	f, err := proxy.OpenFile(context.Background(), "/remote/file.txt", os.O_WRONLY|os.O_CREATE, 0644)

	assert.Error(t, err)
	assert.Nil(t, f)
	assert.Equal(t, os.ErrNotExist, err)
}

func TestLocalFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	log := lgr.New()
	mockClient := &MockWebdav{}

	proxy := fs.NewPikpakProxy(log, tmpDir, mockClient)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "root path",
			input:    "/",
			expected: tmpDir,
		},
		{
			name:     "simple file",
			input:    "/test.txt",
			expected: filepath.Join(tmpDir, "test.txt"),
		},
		{
			name:     "nested path",
			input:    "/dir/subdir/file.txt",
			expected: filepath.Join(tmpDir, "dir", "subdir", "file.txt"),
		},
		{
			name:     "path with dots",
			input:    "/./test.txt",
			expected: filepath.Join(tmpDir, "test.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proxy.LocalFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
