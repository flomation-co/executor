package file_write

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

func textConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

func boolConn(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func Test_WriteFile(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "output.txt")

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", filePath),
		textConn("content", "hello world"),
	})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))
	Expect(result["bytes_written"]).To(Equal(11))

	content, _ := os.ReadFile(filePath)
	Expect(string(content)).To(Equal("hello world"))
}

func Test_WriteFile_Append(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "append.txt")
	os.WriteFile(filePath, []byte("first"), 0644)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", filePath),
		textConn("content", " second"),
		boolConn("append", true),
	})
	Expect(err).To(BeNil())

	content, _ := os.ReadFile(filePath)
	Expect(string(content)).To(Equal("first second"))
}

func Test_WriteFile_CreatesDirectory(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "nested", "file.txt")

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", filePath),
		textConn("content", "nested content"),
	})
	Expect(err).To(BeNil())
	Expect(result["success"]).To(Equal(true))

	content, _ := os.ReadFile(filePath)
	Expect(string(content)).To(Equal("nested content"))
}

func Test_WriteFile_PathTraversal(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("file_path", "/tmp/../etc/evil.txt"),
		textConn("content", "bad"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("path traversal"))
}
