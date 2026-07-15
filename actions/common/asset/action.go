// Package asset is a constant-like node that holds an uploaded file reference
// (a flo:blob: token produced by the editor's upload to POST /api/v1/asset) and
// outputs it, so it can be wired into the file inputs of other nodes — e.g. a
// logo → video_watermark's overlay, or a PSD → psd_rasterise.
//
// It does no work beyond surfacing its stored reference: the token is already a
// media reference every file-consuming action resolves via ResolveToLocalFile,
// so nothing downstream needs to know it came from an upload. The editor renders
// the `file` input as an upload widget (ConnectionTypeFile).
package asset

import (
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "File Asset"
	Description  = "Reference an uploaded file (logo, image, PSD) from other nodes"
	Website      = "https://www.flomation.co"
	Icon         = "paperclip"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "file", Type: core.ConnectionTypeFile, Label: "File", Placeholder: "Upload a file", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "file", Type: core.ConnectionTypeString, Label: "File (media reference)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	c := core.FindConnection("file", inputs)
	if c == nil || c.String() == nil || strings.TrimSpace(*c.String()) == "" {
		return map[string]interface{}{
			"tool_result": "Error: no file has been uploaded to this asset node",
			"success":     false,
			"error":       "no file uploaded",
		}, nil
	}
	ref := strings.TrimSpace(*c.String())
	return map[string]interface{}{
		"tool_result": "File asset ready.",
		"file":        ref,
		"success":     true,
	}, nil
}
