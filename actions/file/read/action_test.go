package file_read

import (
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func Test_ReadFile(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("hello world"), 0644)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", filePath),
	})
	Expect(err).To(BeNil())
	Expect(result["content"]).To(Equal("hello world"))
	Expect(result["file_name"]).To(Equal("test.txt"))
	Expect(result["file_size"]).To(Equal(11))
	Expect(result["success"]).To(Equal(true))
}

func Test_ReadFile_NotFound(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "/nonexistent/file.txt"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("does not exist"))
}

func Test_ReadFile_Directory(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", dir),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("directory"))
}

func Test_ReadFile_PathTraversal(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "/tmp/../etc/passwd"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("path traversal"))
}
