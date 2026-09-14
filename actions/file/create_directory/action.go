// Package file_create_directory creates a folder in the flow's workspace.
package file_create_directory

import (
	"fmt"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Create Directory"
	Description  = "Create a folder in the flow's workspace, including any missing parents."
	Summary      = "Create a folder"
	Website      = "https://www.flomation.co"
	Icon         = "folder+plus"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "directory",
		Type:        core.ConnectionTypeString,
		Label:       "Folder to create, relative to the flow's workspace (for example reports/2026)",
		Placeholder: "reports/2026",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "directory", Type: core.ConnectionTypeString, Label: "Folder"},
	{Name: "created", Type: core.ConnectionTypeBoolean, Label: "Was Newly Created"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	rel, err := file_common.RequirePath(file_common.OptionalString("directory", inputs), "directory")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	// Idempotent: a flow that creates its output folder on every run should not
	// fail on the second one. Reported honestly via `created` so a caller that
	// cares can tell the difference.
	existed := false
	if info, statErr := root.Stat(rel); statErr == nil {
		if !info.IsDir() {
			return file_common.ErrorResult(fmt.Sprintf("%q already exists and is a file, not a folder", rel)), nil
		}
		existed = true
	}

	if err := root.MkdirAll(rel, 0o750); err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}

	summary := fmt.Sprintf("Created folder %s", rel)
	if existed {
		summary = fmt.Sprintf("Folder %s already existed", rel)
	}
	return file_common.OkResult(summary, map[string]interface{}{
		"directory": rel,
		"created":   !existed,
	}), nil
}
