package file_write

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
	Name         = "Write File"
	Description  = "Write content to a file"
	Website      = "https://www.flomation.co"
	Icon         = "file-pen"
	Date         = "23/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "file_path",
		Type:        core.ConnectionTypeString,
		Label:       "File Path",
		Placeholder: "/path/to/output.txt",
		Required:    true,
	},
	{
		Name:        "content",
		Type:        core.ConnectionTypeText,
		Label:       "Content",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "append",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Append to File",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file_path", Type: core.ConnectionTypeString},
	{Name: "bytes_written", Type: core.ConnectionTypeInteger},
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

	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil {
		return nil, fmt.Errorf("content is required")
	}
	content := *contentConn.String()

	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	appendMode := false
	if ac := core.FindConnection("append", inputs); ac != nil && ac.Boolean() != nil {
		appendMode = *ac.Boolean()
	}

	var flag int
	if appendMode {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(filePath, flag, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return map[string]interface{}{
		"file_path":     filePath,
		"bytes_written": n,
		"success":       true,
	}, nil
}
