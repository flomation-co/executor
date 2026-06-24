package databricks_upload_file

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
	Name         = "Databricks Upload File"
	Description  = "Upload a file to a Databricks Unity Catalog Volume"
	Website      = "https://www.flomation.co"
	Icon         = "database+arrow-up"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Volume File Path", Placeholder: "/Volumes/main/default/my_volume/file.csv", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "File Content", Required: true},
	{Name: "is_base64", Type: core.ConnectionTypeBoolean, Label: "Content is Base64", Placeholder: "Enable for binary files"},
	{Name: "overwrite", Type: core.ConnectionTypeBoolean, Label: "Overwrite if Exists"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path"},
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
	contentConn := core.FindConnection("content", inputs)
	if contentConn == nil || contentConn.String() == nil || *contentConn.String() == "" {
		return databricks.ErrorResult("content is required"), nil
	}
	content := *contentConn.String()

	data := []byte(content)
	if conn := core.FindConnection("is_base64", inputs); conn != nil && conn.Boolean() != nil && *conn.Boolean() {
		decoded, derr := base64.StdEncoding.DecodeString(content)
		if derr != nil {
			return databricks.ErrorResult(fmt.Sprintf("content is not valid base64: %s", derr)), nil
		}
		data = decoded
	}

	apiPath := "/api/2.0/fs/files" + databricks.EncodePath(path)
	overwrite := true // default to overwrite
	if conn := core.FindConnection("overwrite", inputs); conn != nil && conn.Boolean() != nil {
		overwrite = *conn.Boolean()
	}
	if overwrite {
		apiPath += "?overwrite=true"
	}

	resp, err := databricks.ExecuteRaw(host, token, http.MethodPut, apiPath, "application/octet-stream", data)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to upload file: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Uploaded %d bytes to %s", len(data), path),
		"path":        path,
		"size":        len(data),
		"success":     true,
		"error":       "",
	}, nil
}
