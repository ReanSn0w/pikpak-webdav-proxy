package utils_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDirectoryProxy - мок для DirectoryProxy
type MockDirectoryProxy struct {
	mock.Mock
}

func (m *MockDirectoryProxy) Readdir(ctx context.Context, name string) ([]os.FileInfo, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

// TestClose проверяет, что Close возвращает nil
func TestDirClose(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	df := createDirectoryFile(mockProxy, "test")

	err := df.Close()
	assert.NoError(t, err)
}

// TestRead проверяет, что Read возвращает 0 и os.ErrInvalid
func TestRead(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	df := createDirectoryFile(mockProxy, "test")

	n, err := df.Read(make([]byte, 10))
	assert.Equal(t, 0, n)
	assert.Equal(t, os.ErrInvalid, err)
}

// TestSeek проверяет, что Seek возвращает 0 и os.ErrInvalid
func TestSeek(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	df := createDirectoryFile(mockProxy, "test")

	offset, err := df.Seek(0, 0)
	assert.Equal(t, int64(0), offset)
	assert.Equal(t, os.ErrInvalid, err)
}

// TestWrite проверяет, что Write возвращает 0 и os.ErrInvalid
func TestWrite(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	df := createDirectoryFile(mockProxy, "test")

	n, err := df.Write([]byte("test"))
	assert.Equal(t, 0, n)
	assert.Equal(t, os.ErrInvalid, err)
}

// TestStat проверяет, что Stat возвращает корректный FileInfo
func TestDirStat(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	fileInfo := createMockFileInfo("test", true)
	df := createDirectoryFileWithFileInfo(mockProxy, "test", fileInfo)

	stat, err := df.Stat()
	assert.NoError(t, err)
	assert.Equal(t, fileInfo, stat)
}

// TestReaddirWithCountZero проверяет чтение всех файлов при count <= 0
func TestReaddirWithCountZero(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
		createMockFileInfo("file3", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	result, err := df.Readdir(0)
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, files, result)

	mockProxy.AssertCalled(t, "Readdir", mock.Anything, "test")
}

// TestReaddirWithPositiveCount проверяет чтение определенного количества файлов
func TestReaddirWithPositiveCount(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
		createMockFileInfo("file3", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Первый вызов с count=2
	result1, err1 := df.Readdir(2)
	assert.NoError(t, err1)
	assert.Len(t, result1, 2)
	assert.Equal(t, files[0:2], result1)

	// Второй вызов с count=2 (последний элемент + EOF)
	result2, err2 := df.Readdir(2)
	assert.Equal(t, io.EOF, err2)
	assert.Len(t, result2, 1)
	assert.Equal(t, files[2:3], result2)
}

// TestReaddirCaching проверяет, что файлы кешируются после первого вызова
func TestReaddirCaching(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Первый вызов - возвращает первый файл без EOF (есть второй)
	result1, err1 := df.Readdir(1)
	assert.NoError(t, err1)
	assert.Len(t, result1, 1)

	// Второй вызов - возвращает второй файл с EOF (это последний)
	result2, err2 := df.Readdir(1)
	assert.Equal(t, io.EOF, err2)
	assert.Len(t, result2, 1)

	// Проверяем, что proxy был вызван только один раз
	mockProxy.AssertNumberOfCalls(t, "Readdir", 1)
}

// TestReaddirError проверяет, что ошибки от proxy передаются корректно
func TestReaddirError(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	expectedErr := errors.New("read error")

	mockProxy.On("Readdir", mock.Anything, "test").Return(nil, expectedErr)
	df := createDirectoryFile(mockProxy, "test")

	result, err := df.Readdir(10)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

// TestReaddirMultipleCalls проверяет последовательное чтение файлов
func TestReaddirMultipleCalls(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
		createMockFileInfo("file3", false),
		createMockFileInfo("file4", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Первый вызов
	result1, err1 := df.Readdir(2)
	assert.NoError(t, err1)
	assert.Len(t, result1, 2)

	// Второй вызов
	result2, err2 := df.Readdir(2)
	assert.Equal(t, io.EOF, err2) // Последний вызов возвращает EOF
	assert.Len(t, result2, 2)

	// Третий вызов - файлов больше нет, возвращаем пусто с EOF
	result3, err3 := df.Readdir(2)
	assert.Equal(t, io.EOF, err3)
	assert.Len(t, result3, 0)
}

// TestReaddirNegativeCount проверяет чтение всех файлов при count < 0
func TestReaddirNegativeCount(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	result, err := df.Readdir(-1)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, files, result)
}

// TestReaddirEmptyDirectory проверяет чтение пустой директории
func TestReaddirEmptyDirectory(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	emptyFiles := []os.FileInfo{}

	mockProxy.On("Readdir", mock.Anything, "test").Return(emptyFiles, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Даже для пустой директории на первый вызов вернется EOF
	result, err := df.Readdir(10)
	assert.Equal(t, io.EOF, err)
	assert.Len(t, result, 0)
}

// TestReaddirLargeCount проверяет запрос большего количества файлов, чем есть
func TestReaddirLargeCount(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Запрашиваем 100, но файлов только 2
	result, err := df.Readdir(100)
	assert.Equal(t, io.EOF, err)
	assert.Len(t, result, 2)
	assert.Equal(t, files, result)
}

// TestReaddirResetPosition проверяет позицию после count <= 0
func TestReaddirResetPosition(t *testing.T) {
	mockProxy := new(MockDirectoryProxy)
	files := []os.FileInfo{
		createMockFileInfo("file1", false),
		createMockFileInfo("file2", false),
		createMockFileInfo("file3", false),
	}

	mockProxy.On("Readdir", mock.Anything, "test").Return(files, nil)
	df := createDirectoryFile(mockProxy, "test")

	// Читаем одного файла
	_, _ = df.Readdir(1)

	// Читаем всех оставшихся
	result, _ := df.Readdir(0)
	assert.Len(t, result, 2)
	assert.Equal(t, files[1:3], result)
}

// Helper functions

func createDirectoryFile(proxy utils.DirectoryProxy, name string) *utils.DirectoryFile {
	return createDirectoryFileWithFileInfo(proxy, name, createMockFileInfo(name, true))
}

func createDirectoryFileWithFileInfo(proxy utils.DirectoryProxy, name string, fileInfo os.FileInfo) *utils.DirectoryFile {
	return &utils.DirectoryFile{
		Name:      name,
		Proxy:     proxy,
		FileInfo:  fileInfo,
		Files:     nil,
		FileIndex: 0,
	}
}

func createMockFileInfo(name string, isDir bool) os.FileInfo {
	return &mockFileInfo{
		name:  name,
		isDir: isDir,
	}
}

// mockFileInfo - простой мок для os.FileInfo
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
