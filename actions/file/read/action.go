package file_read

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read File"
	Description  = "Read the contents of a file"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction

	maxFileSize = 5 << 20 // 5 MB
)

var Inputs = [...]core.Connection{
	{
		Name:        "file_path",
		Type:        core.ConnectionTypeString,
		Label:       "File Path",
		Placeholder: "/path/to/file.txt",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString},
	{Name: "file_name", Type: core.ConnectionTypeString},
	{Name: "file_size", Type: core.ConnectionTypeInteger},
	{Name: "success", Type: core.ConnectionTypeBoolean},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	pathConn := core.FindConnection("file_path", inputs)
	if pathConn == nil || pathConn.String() == nil || *pathConn.String() == "" {
		return nil, fmt.Errorf("file_path is required")
	}
	filePath := *pathConn.String()

	if strings.Contains(filePath, "..") {
		return nil, fmt.Errorf("file path must not contain path traversal")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", maxFileSize)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return map[string]interface{}{
		"content":   string(content),
		"file_name": filepath.Base(filePath),
		"file_size": int(info.Size()),
		"success":   true,
	}, nil
}
