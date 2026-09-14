// Package file_read reads a text file from the flow's workspace.
package file_read

import (
	"fmt"
	"path"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read File"
	Description  = "Read a text file from the flow's workspace."
	Summary      = "Read a file"
	Website      = "https://www.flomation.co"
	Icon         = "file+arrow-up"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "File Contents"},
	{Name: "file_name", Type: core.ConnectionTypeString, Label: "File Name"},
	{Name: "file_size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	rel, err := file_common.RequirePath(file_common.OptionalString("file_path", inputs), "file_path")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(rel)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}
	if info.IsDir() {
		return file_common.ErrorResult(fmt.Sprintf("%q is a directory, not a file — use List Files to see what is in it", rel)), nil
	}
	if info.Size() > file_common.MaxFileSize {
		return file_common.ErrorResult(fmt.Sprintf(
			"%q is %d bytes, over the %d byte limit for a single read", rel, info.Size(), file_common.MaxFileSize)), nil
	}

	content, err := root.ReadFile(rel)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}

	// The contents go IN tool_result, not just a byte count: an agent handed
	// "read 412 bytes" has to make a second call to do anything with it.
	return file_common.OkResult(fmt.Sprintf("%s (%d bytes):\n%s", rel, info.Size(), string(content)), map[string]interface{}{
		"content":   string(content),
		"file_name": path.Base(rel),
		"file_size": int(info.Size()),
	}), nil
}
