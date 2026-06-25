package databricks_download_file

import (
	"encoding/base64"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks Download File"
	Description  = "Download a file from a Databricks Unity Catalog Volume"
	Website      = "https://www.flomation.co"
	Icon         = "database+arrow-down"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Volume File Path", Placeholder: "/Volumes/main/default/my_volume/file.csv", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content (Base64)"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	path, err := databricks.RequiredString("path", inputs)
	if err != nil {
		return nil, err
	}

	apiPath := "/api/2.0/fs/files" + databricks.EncodePath(path)
	// ExecuteRaw buffers the body via readCapped (common.go), which enforces the
	// maxResponseBody cap (50 MB) and returns an error rather than silently
	// truncating — so an oversized file soft-fails here instead of OOMing the
	// executor or producing a corrupt (truncated) base64 result below.
	resp, err := databricks.ExecuteRaw(host, token, http.MethodGet, apiPath, "", nil)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to download file: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	encoded := base64.StdEncoding.EncodeToString(resp.Body)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Downloaded %d bytes from %s", len(resp.Body), path),
		"content":     encoded,
		"size":        len(resp.Body),
		"success":     true,
		"error":       "",
	}, nil
}
