// Package file_info reports whether something exists in the flow's workspace.
package file_info

import (
	"fmt"
	"path"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "File Info"
	Description  = "Check whether a file or folder exists in the flow's workspace, and read its size and date."
	Summary      = "Check a file exists"
	Website      = "https://www.flomation.co"
	Icon         = "file+circle-info"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "path",
		Type:        core.ConnectionTypeString,
		Label:       "File or folder to check, relative to the flow's workspace",
		Placeholder: "reports/summary.txt",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "exists", Type: core.ConnectionTypeBoolean, Label: "Exists"},
	{Name: "is_directory", Type: core.ConnectionTypeBoolean, Label: "Is a Folder"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "modified", Type: core.ConnectionTypeString, Label: "Last Modified (UTC)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	rel, err := file_common.RequirePath(file_common.OptionalString("path", inputs), "path")
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
		// Not existing is an ANSWER here, not a failure — the whole point of the
		// action is to ask. An escape attempt is still an error.
		if file_common.IsNotExist(err) {
			return file_common.OkResult(fmt.Sprintf("%s does not exist in the workspace", rel), map[string]interface{}{
				"exists":       false,
				"is_directory": false,
				"size":         0,
				"modified":     "",
				"name":         path.Base(rel),
			}), nil
		}
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}

	entry := file_common.EntryFrom(rel, info)
	kind := "file"
	if entry.IsDir {
		kind = "folder"
	}
	return file_common.OkResult(
		fmt.Sprintf("%s exists (%s, %d bytes, modified %s)", rel, kind, entry.Size, entry.Modified),
		map[string]interface{}{
			"exists":       true,
			"is_directory": entry.IsDir,
			"size":         int(entry.Size),
			"modified":     entry.Modified,
			"name":         entry.Name,
		}), nil
}
