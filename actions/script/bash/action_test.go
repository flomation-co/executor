package script_bash

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func textConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

func intConn(name string, value int64) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: value}
}

func Test_SimpleEcho(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo 'hello world'"),
	})
	Expect(err).To(BeNil())
	Expect(result["stdout"]).To(Equal("hello world"))
	Expect(result["exit_code"]).To(Equal(0))
	Expect(result["success"]).To(Equal(true))
}

func Test_StderrCapture(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo 'error output' >&2"),
	})
	Expect(err).To(BeNil())
	Expect(result["stderr"]).To(Equal("error output"))
	Expect(result["stdout"]).To(Equal(""))
	Expect(result["exit_code"]).To(Equal(0))
}

func Test_NonZeroExitCode(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "exit 42"),
	})
	Expect(err).To(BeNil())
	Expect(result["exit_code"]).To(Equal(42))
	Expect(result["success"]).To(Equal(false))
}

func Test_Timeout(t *testing.T) {
	RegisterTestingT(t)

	start := time.Now()
	_, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "sleep 60"),
		{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Value: "2"},
	})
	elapsed := time.Since(start)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("timeout"))
	Expect(elapsed.Seconds()).To(BeNumerically("<", 10))
}

func Test_WorkingDirectory(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	resolvedDir, _ := filepath.EvalSymlinks(dir)

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "pwd"),
		strConn("working_directory", dir),
	})
	Expect(err).To(BeNil())
	Expect(result["stdout"]).To(Equal(resolvedDir))
}

func Test_EmptyScript(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "   "),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("script is required"))
}

func Test_MissingScript(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("script is required"))
}

func Test_InvalidWorkingDirectory(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo test"),
		strConn("working_directory", "/nonexistent/path/that/does/not/exist"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("does not exist"))
}

func Test_PathTraversal(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo test"),
		strConn("working_directory", "/tmp/../etc"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("path traversal"))
}

func Test_EnvironmentIsolation(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo $HOME"),
	})
	Expect(err).To(BeNil())
	// HOME should be the temp dir, not the real home
	home := result["stdout"].(string)
	Expect(home).ToNot(Equal(os.Getenv("HOME")))
	Expect(home).To(ContainSubstring("flomation-bash"))
}

func Test_ScriptCanWriteFiles(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo 'test content' > output.txt && cat output.txt"),
		strConn("working_directory", dir),
	})
	Expect(err).To(BeNil())
	Expect(result["stdout"]).To(Equal("test content"))
	Expect(result["success"]).To(Equal(true))

	// Verify the file was actually created
	content, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	Expect(err).To(BeNil())
	Expect(string(content)).To(ContainSubstring("test content"))
}

func Test_MultilineScript(t *testing.T) {
	RegisterTestingT(t)

	script := `#!/bin/bash
A=5
B=10
RESULT=$((A + B))
echo $RESULT`

	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", script),
	})
	Expect(err).To(BeNil())
	Expect(result["stdout"]).To(Equal(strconv.Itoa(15)))
	Expect(result["success"]).To(Equal(true))
}

func Test_DefaultTimeout(t *testing.T) {
	RegisterTestingT(t)

	// Script that finishes quickly — just verify it runs with no timeout input
	result, err := Execute(nil, nil, []*core.Connection{
		textConn("script", "echo 'fast'"),
	})
	Expect(err).To(BeNil())
	Expect(result["stdout"]).To(Equal("fast"))
}
