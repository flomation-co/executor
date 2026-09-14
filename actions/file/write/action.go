// Package file_write writes a text file into the flow's workspace.
package file_write

import (
	"fmt"
	"os"
	"path"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Write File"
	Description  = "Write or append a text file in the flow's workspace."
	Summary      = "Write a file"
	Website      = "https://www.flomation.co"
	Icon         = "file+arrow-down"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "file_path",
		Type:        core.ConnectionTypeString,
		Label:       "File path, relative to the flow's workspace (for example reports/summary.txt)",
		Placeholder: "reports/summary.txt",
		Required:    true,
	},
	{
		Name:     "content",
		Type:     core.ConnectionTypeText,
		Label:    "Content",
		Required: true,
	},
	{
		Name:  "append",
		Type:  core.ConnectionTypeBoolean,
		Label: "Append to the file instead of replacing it",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file_path", Type: core.ConnectionTypeString, Label: "File Path"},
	{Name: "bytes_written", Type: core.ConnectionTypeInteger, Label: "Bytes Written"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	rel, err := file_common.RequirePath(file_common.OptionalString("file_path", inputs), "file_path")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}

	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil {
		return file_common.ErrorResult("content is required"), nil
	}
	content := *contentConn.String()

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	// Create the parent inside the root, so "reports/summary.txt" works without
	// a separate Create Directory step. MkdirAll on the root is confined the
	// same way every other operation is.
	if parent := path.Dir(rel); parent != "." {
		if err := root.MkdirAll(parent, 0o750); err != nil {
			return file_common.ErrorResult(file_common.DescribeError(err, parent).Error()), nil
		}
	}

	appendMode := file_common.OptionalBool("append", inputs)
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	verb := "Wrote"
	if appendMode {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		verb = "Appended to"
	}

	f, err := root.OpenFile(rel, flag, 0o640)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}
	defer func() { _ = f.Close() }()

	n, err := f.WriteString(content)
	if err != nil {
		return file_common.ErrorResult(fmt.Sprintf("could not write %q: %v", rel, err)), nil
	}

	return file_common.OkResult(fmt.Sprintf("%s %s (%d bytes)", verb, rel, n), map[string]interface{}{
		"file_path":     rel,
		"bytes_written": n,
	}), nil
}
