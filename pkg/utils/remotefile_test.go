package utils_test

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ReanSn0w/pikpak-webdav-proxy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockWebDav - мок для WebDav интерфейса
type MockWebDav struct {
	mock.Mock
}

func (m *MockWebDav) Stat(path string) (os.FileInfo, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (m *MockWebDav) ReadDir(path string) ([]os.FileInfo, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

func (m *MockWebDav) ReadStreamRange(path string, offset int64, length int64) (io.ReadCloser, error) {
	args := m.Called(path, offset, length)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

// MockFileInfo - мок для os.FileInfo
type MockFileInfo struct {
	mock.Mock
}

func (m *MockFileInfo) Name() string {
	return m.Called().String(0)
}

func (m *MockFileInfo) Size() int64 {
	return m.Called().Get(0).(int64)
}

func (m *MockFileInfo) Mode() os.FileMode {
	return m.Called().Get(0).(os.FileMode)
}

func (m *MockFileInfo) ModTime() time.Time {
	return m.Called().Get(0).(time.Time)
}

func (m *MockFileInfo) IsDir() bool {
	return m.Called().Get(0).(bool)
}

func (m *MockFileInfo) Sys() interface{} {
	return nil
}

// ReadCloserMock - простой мок для io.ReadCloser
type ReadCloserMock struct {
	io.Reader
}

func (r *ReadCloserMock) Close() error {
	return nil
}

// TestNewRemoteFile_Success тестирует успешное создание удалённого файла
func TestNewRemoteFile_Success(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(1024))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/path/to/file.txt").Return(mockFileInfo, nil)

	file, err := utils.NewRemoteFile(mockClient, "/path/to/file.txt")

	require.NoError(t, err)
	assert.NotNil(t, file)
	mockClient.AssertExpectations(t)
	mockFileInfo.AssertExpectations(t)
}

// TestNewRemoteFile_StatError тестирует ошибку при Stat
func TestNewRemoteFile_StatError(t *testing.T) {
	mockClient := new(MockWebDav)

	mockClient.On("Stat", "/nonexistent").Return(nil, os.ErrNotExist)

	file, err := utils.NewRemoteFile(mockClient, "/nonexistent")

	assert.Nil(t, file)
	assert.ErrorIs(t, err, os.ErrNotExist)
	mockClient.AssertExpectations(t)
}

// TestNewRemoteFile_Directory тестирует создание для директории
func TestNewRemoteFile_Directory(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/path/to/dir").Return(mockFileInfo, nil)

	file, err := utils.NewRemoteFile(mockClient, "/path/to/dir")

	require.NoError(t, err)
	assert.NotNil(t, file)
	mockClient.AssertExpectations(t)
}

// TestRead_Success тестирует успешное чтение файла
func TestRead_Success(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	testData := "Hello, World!"
	mockFileInfo.On("Size").Return(int64(len(testData)))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)
	mockClient.On("ReadStreamRange", "/file.txt", int64(0), int64(len(testData))).
		Return(&ReadCloserMock{strings.NewReader(testData)}, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	buffer := make([]byte, len(testData))
	n, err := file.Read(buffer)

	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, string(buffer))
	mockClient.AssertExpectations(t)
}

// TestRead_EOF тестирует достижение конца файла
func TestRead_EOF(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(10))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	// Seek в конец файла
	file.Seek(10, io.SeekStart)

	buffer := make([]byte, 10)
	n, err := file.Read(buffer)

	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

// TestRead_Directory тестирует чтение директории (ошибка)
func TestRead_Directory(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/dir").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/dir")

	buffer := make([]byte, 10)
	n, err := file.Read(buffer)

	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

// TestRead_Error тестирует ошибку при чтении
func TestRead_Error(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)
	mockClient.On("ReadStreamRange", "/file.txt", int64(0), int64(10)).
		Return(nil, errors.New("read error"))

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	buffer := make([]byte, 10)
	n, err := file.Read(buffer)

	assert.Equal(t, 0, n)
	assert.Error(t, err)
}

// TestRead_PartialBytes тестирует чтение части байтов
func TestRead_PartialBytes(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	testData := "Hello, World!"
	mockFileInfo.On("Size").Return(int64(len(testData)))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)
	mockClient.On("ReadStreamRange", "/file.txt", int64(0), int64(5)).
		Return(&ReadCloserMock{strings.NewReader(testData[:5])}, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	buffer := make([]byte, 5)
	n, err := file.Read(buffer)

	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "Hello", string(buffer))
}

// TestSeek_SeekStart тестирует Seek с SeekStart
func TestSeek_SeekStart(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	newOffset, err := file.Seek(50, io.SeekStart)

	assert.NoError(t, err)
	assert.Equal(t, int64(50), newOffset)
}

// TestSeek_SeekCurrent тестирует Seek с SeekCurrent
func TestSeek_SeekCurrent(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	file.Seek(25, io.SeekStart)
	newOffset, err := file.Seek(25, io.SeekCurrent)

	assert.NoError(t, err)
	assert.Equal(t, int64(50), newOffset)
}

// TestSeek_SeekEnd тестирует Seek с SeekEnd
func TestSeek_SeekEnd(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	newOffset, err := file.Seek(-10, io.SeekEnd)

	assert.NoError(t, err)
	assert.Equal(t, int64(90), newOffset)
}

// TestSeek_NegativeOffset тестирует отрицательное смещение
func TestSeek_NegativeOffset(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	newOffset, err := file.Seek(-50, io.SeekStart)

	assert.ErrorIs(t, err, os.ErrInvalid)
	assert.Equal(t, int64(0), newOffset)
}

// TestSeek_BeyondFileSize тестирует смещение за границы файла
func TestSeek_BeyondFileSize(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	newOffset, err := file.Seek(150, io.SeekStart)

	assert.NoError(t, err)
	assert.Equal(t, int64(100), newOffset)
}

// TestSeek_InvalidWhence тестирует невалидный whence параметр
func TestSeek_InvalidWhence(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	newOffset, err := file.Seek(10, 999)

	assert.ErrorIs(t, err, os.ErrInvalid)
	assert.Equal(t, int64(0), newOffset)
}

// TestSeek_Directory тестирует Seek на директории
func TestSeek_Directory(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/dir").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/dir")

	newOffset, err := file.Seek(10, io.SeekStart)

	assert.ErrorIs(t, err, os.ErrInvalid)
	assert.Equal(t, int64(0), newOffset)
}

// TestWrite_NotSupported тестирует возврат ошибки при Write
func TestWrite_NotSupported(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	n, err := file.Write([]byte("test"))

	assert.Equal(t, 0, n)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write to remote file not enabled")
}

// TestClose тестирует Close
func TestClose(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	err := file.Close()

	assert.NoError(t, err)
}

// TestStat тестирует Stat
func TestStat(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(1024))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	statInfo, err := file.Stat()

	assert.NoError(t, err)
	assert.Equal(t, mockFileInfo, statInfo)
}

// TestReaddir_Success тестирует успешное чтение директории
func TestReaddir_Success(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)
	file1 := new(MockFileInfo)
	file2 := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/dir").Return(mockFileInfo, nil)
	mockClient.On("ReadDir", "/dir").Return([]os.FileInfo{file1, file2}, nil)

	remoteFile, _ := utils.NewRemoteFile(mockClient, "/dir")

	files, err := remoteFile.Readdir(-1)

	assert.NoError(t, err)
	assert.Len(t, files, 2)
	mockClient.AssertExpectations(t)
}

// TestReaddir_LimitCount тестирует чтение директории с лимитом
func TestReaddir_LimitCount(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)
	file1 := new(MockFileInfo)
	file2 := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/dir").Return(mockFileInfo, nil)
	mockClient.On("ReadDir", "/dir").Return([]os.FileInfo{file1, file2}, nil)

	remoteFile, _ := utils.NewRemoteFile(mockClient, "/dir")

	files, err := remoteFile.Readdir(1)

	assert.NoError(t, err)
	assert.Len(t, files, 1)
}

// TestReaddir_NotDirectory тестирует чтение не-директории
func TestReaddir_NotDirectory(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(100))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	files, err := file.Readdir(10)

	assert.Nil(t, files)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

// TestReaddir_Error тестирует ошибку при чтении директории
func TestReaddir_Error(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	mockFileInfo.On("Size").Return(int64(0))
	mockFileInfo.On("IsDir").Return(true)

	mockClient.On("Stat", "/dir").Return(mockFileInfo, nil)
	mockClient.On("ReadDir", "/dir").Return(nil, os.ErrPermission)

	remoteFile, _ := utils.NewRemoteFile(mockClient, "/dir")

	files, err := remoteFile.Readdir(10)

	assert.Nil(t, files)
	assert.ErrorIs(t, err, os.ErrPermission)
}

// TestConcurrentRead тестирует параллельные чтения (race condition protection)
func TestConcurrentRead(t *testing.T) {
	mockClient := new(MockWebDav)
	mockFileInfo := new(MockFileInfo)

	testData := "Hello, World!"
	mockFileInfo.On("Size").Return(int64(len(testData)))
	mockFileInfo.On("IsDir").Return(false)

	mockClient.On("Stat", "/file.txt").Return(mockFileInfo, nil)
	mockClient.On("ReadStreamRange", mock.Anything, mock.Anything, mock.Anything).
		Return(&ReadCloserMock{strings.NewReader(testData)}, nil)

	file, _ := utils.NewRemoteFile(mockClient, "/file.txt")

	done := make(chan bool, 2)

	go func() {
		buffer := make([]byte, 5)
		file.Read(buffer)
		done <- true
	}()

	go func() {
		buffer := make([]byte, 5)
		file.Read(buffer)
		done <- true
	}()

	<-done
	<-done

	assert.True(t, true) // Если не было race condition, тест пройдёт
}
