// Package file_copy copies a file inside the flow's workspace.
package file_copy

import (
	"fmt"
	"io"
	"os"
	"path"

	core "flomation.app/automate/executor"
	file_common "flomation.app/automate/executor/actions/file"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Copy File"
	Description  = "Copy a file to another path within the flow's workspace."
	Summary      = "Copy a file"
	Website      = "https://www.flomation.co"
	Icon         = "file+copy"
	Date         = "14/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "source",
		Type:        core.ConnectionTypeString,
		Label:       "File to copy, relative to the flow's workspace",
		Placeholder: "reports/summary.txt",
		Required:    true,
	},
	{
		Name:        "destination",
		Type:        core.ConnectionTypeString,
		Label:       "Where to copy it to, relative to the flow's workspace",
		Placeholder: "archive/summary.txt",
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
	{Name: "bytes_copied", Type: core.ConnectionTypeInteger, Label: "Bytes Copied"},
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

	info, err := root.Stat(source)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, source).Error()), nil
	}
	// Copying a tree is a different job with different failure modes (partial
	// copies, loops); Move handles folders, and this stays a file operation
	// rather than half-implementing recursion.
	if info.IsDir() {
		return file_common.ErrorResult(fmt.Sprintf("%q is a folder — this action copies files. Use Move File to relocate a folder", source)), nil
	}
	if info.Size() > file_common.MaxFileSize {
		return file_common.ErrorResult(fmt.Sprintf(
			"%q is %d bytes, over the %d byte limit", source, info.Size(), file_common.MaxFileSize)), nil
	}

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

	in, err := root.Open(source)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, source).Error()), nil
	}
	defer func() { _ = in.Close() }()

	out, err := root.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return file_common.ErrorResult(file_common.DescribeError(err, destination).Error()), nil
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, in)
	if err != nil {
		return file_common.ErrorResult(fmt.Sprintf("could not copy %q to %q: %v", source, destination, err)), nil
	}

	return file_common.OkResult(fmt.Sprintf("Copied %s to %s (%d bytes)", source, destination, n), map[string]interface{}{
		"source":       source,
		"destination":  destination,
		"bytes_copied": int(n),
	}), nil
}
