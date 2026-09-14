// Package file_move moves or renames something inside the flow's workspace.
package file_move

import (
	"fmt"
	"path"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Move File"
	Description  = "Move or rename a file or folder within the flow's workspace."
	Summary      = "Move or rename a file"
	Website      = "https://www.flomation.co"
	Icon         = "file+arrow-right"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "source",
		Type:        core.ConnectionTypeString,
		Label:       "File or folder to move, relative to the flow's workspace",
		Placeholder: "incoming/report.csv",
		Required:    true,
	},
	{
		Name:        "destination",
		Type:        core.ConnectionTypeString,
		Label:       "New path, relative to the flow's workspace",
		Placeholder: "processed/report.csv",
		Required:    true,
	},
	{
		Name:  "overwrite",
		Type:  core.ConnectionTypeBoolean,
		Label: "Replace the destination if it already exists",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source"},
	{Name: "destination", Type: core.ConnectionTypeString, Label: "Destination"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	source, err := file_common.RequirePath(file_common.OptionalString("source", inputs), "source")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	destination, err := file_common.RequirePath(file_common.OptionalString("destination", inputs), "destination")
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	if source == destination {
		return file_common.ErrorResult("the source and destination are the same path"), nil
	}

	root, err := file_common.Workspace()
	if err != nil {
		return file_common.ErrorResult(err.Error()), nil
	}
	defer func() { _ = root.Close() }()

	if _, err := root.Stat(source); err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, source).Error()), nil
	}

	// Rename will happily clobber an existing destination, which is a silent
	// data loss an agent would never report. Check first unless told otherwise.
	if !file_common.OptionalBool("overwrite", inputs) {
		if _, err := root.Stat(destination); err == nil {
			return file_common.ErrorResult(fmt.Sprintf(
				"%q already exists — tick Replace the destination if you mean to overwrite it", destination)), nil
		}
	}

	if parent := path.Dir(destination); parent != "." {
		if err := root.MkdirAll(parent, 0o750); err != nil {
			return file_common.ErrorResult(file_common.DescribeError(err, parent).Error()), nil
		}
	}

	if err := root.Rename(source, destination); err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, source+" -> "+destination).Error()), nil
	}

	return file_common.OkResult(fmt.Sprintf("Moved %s to %s", source, destination), map[string]interface{}{
		"source":      source,
		"destination": destination,
	}), nil
}
