package databricks_list_files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks List Files"
	Description  = "List the contents of a directory in a Databricks Unity Catalog Volume"
	Website      = "https://www.flomation.co"
	Icon         = "database+folder"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Directory Path", Placeholder: "/Volumes/main/default/my_volume", Required: true},
	{Name: "page_token", Type: core.ConnectionTypeString, Label: "Page Token", Placeholder: "Optional — for the next page of results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "files", Type: core.ConnectionTypeObject, Label: "Files (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_page_token", Type: core.ConnectionTypeString, Label: "Next Page Token"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type listDirectoryResponse struct {
	Contents      []map[string]interface{} `json:"contents"`
	NextPageToken string                   `json:"next_page_token"`
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

	apiPath := "/api/2.0/fs/directories" + databricks.EncodePath(path)
	if pageToken := databricks.OptionalString("page_token", inputs); pageToken != "" {
		apiPath += "?" + url.Values{"page_token": {pageToken}}.Encode()
	}

	resp, err := databricks.ExecuteAPI(host, token, http.MethodGet, apiPath, nil)
	if err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to list files: %s", err)), nil
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return databricks.ErrorResult(err.Error()), nil
	}

	var out listDirectoryResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return databricks.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}
	if out.Contents == nil {
		out.Contents = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Found %d item(s) in %s", len(out.Contents), path),
		"files":           out.Contents,
		"count":           len(out.Contents),
		"next_page_token": out.NextPageToken,
		"success":         true,
		"error":           "",
	}, nil
}
