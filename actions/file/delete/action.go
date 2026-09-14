// Package file_delete removes a file or folder from the flow's workspace.
package file_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Delete File"
	Description  = "Delete a file or folder from the flow's workspace. Folders need the recursive option."
	Summary      = "Delete a file or folder"
	Website      = "https://www.flomation.co"
	Icon         = "file+trash"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "path",
		Type:        core.ConnectionTypeString,
		Label:       "File or folder to delete, relative to the flow's workspace",
		Placeholder: "reports/old.csv",
		Required:    true,
	},
	{
		Name:  "recursive",
		Type:  core.ConnectionTypeBoolean,
		Label: "Delete a folder and everything inside it (required to delete a non-empty folder)",
	},
	{
		Name:  "ignore_missing",
		Type:  core.ConnectionTypeBoolean,
		Label: "Succeed quietly if it is not there",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Something Was Deleted"},
	{Name: "was_directory", Type: core.ConnectionTypeBoolean, Label: "Was a Folder"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	raw := file_common.OptionalString("path", inputs)
	rel, err := file_common.RequirePath(raw, "path")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	// RequirePath already refuses the workspace root, which is the one target
	// that would wipe the execution out from under the flow. Said plainly here
	// because "." and "/" are exactly what a confused agent reaches for.
	if rel == "." {
		return file_common.ErrorResult("refusing to delete the workspace itself — name a file or folder inside it"), nil
	}

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(rel)
	if err != nil {
		if file_common.IsNotExist(err) {
			if file_common.OptionalBool("ignore_missing", inputs) {
				return file_common.OkResult(fmt.Sprintf("%s was already absent", rel), map[string]interface{}{
					"path": rel, "deleted": false, "was_directory": false,
				}), nil
			}
			return file_common.ErrorResult(fmt.Sprintf("%q does not exist in the flow's workspace", rel)), nil
		}
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}

	recursive := file_common.OptionalBool("recursive", inputs)

	// A non-empty folder needs an explicit decision. Deleting a tree because a
	// path happened to be a directory is the kind of mistake an agent makes once
	// and cannot undo, so the caller has to have meant it.
	if info.IsDir() && !recursive {
		if err := root.Remove(rel); err != nil {
			return file_common.ErrorResult(fmt.Sprintf(
				"%q is a folder and is not empty. Tick \"Delete a folder and everything inside it\" if that is what you want", rel)), nil
		}
		return file_common.OkResult(fmt.Sprintf("Deleted empty folder %s", rel), map[string]interface{}{
			"path": rel, "deleted": true, "was_directory": true,
		}), nil
	}

	if info.IsDir() {
		if err := root.RemoveAll(rel); err != nil {
			return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
		}
		return file_common.OkResult(fmt.Sprintf("Deleted folder %s and its contents", rel), map[string]interface{}{
			"path": rel, "deleted": true, "was_directory": true,
		}), nil
	}

	if err := root.Remove(rel); err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, rel).Error()), nil
	}
	return file_common.OkResult(fmt.Sprintf("Deleted %s", rel), map[string]interface{}{
		"path": rel, "deleted": true, "was_directory": false,
	}), nil
}
