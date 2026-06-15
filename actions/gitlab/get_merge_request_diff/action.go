package gitlab_get_merge_request_diff

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Get Merge Request Diff"
	Description  = "Retrieve the diff and changed files for a GitLab merge request"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+file-lines"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "changes", Type: core.ConnectionTypeObject, Label: "Changes (JSON)"},
	{Name: "changes_count", Type: core.ConnectionTypeInteger, Label: "Number of Changed Files"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gitlab.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := gitlab.GetBaseURL(inputs)
	projectID, err := gitlab.GetProjectID(inputs)
	if err != nil {
		return nil, err
	}
	iid, err := gitlab.RequiredString("merge_request_iid", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/merge_requests/%s/changes", iid), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get diff: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var result struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("MR !%s has %d changed file(s):\n\n", iid, len(result.Changes))
	for _, c := range result.Changes {
		status := "modified"
		if c.NewFile {
			status = "added"
		} else if c.DeletedFile {
			status = "deleted"
		} else if c.RenamedFile {
			status = fmt.Sprintf("renamed from %s", c.OldPath)
		}
		summary += fmt.Sprintf("--- %s (%s) ---\n%s\n\n", c.NewPath, status, c.Diff)
	}

	var changes []interface{}
	_ = json.Unmarshal(resp.Body, &struct {
		C *[]interface{} `json:"changes"`
	}{C: &changes})

	return map[string]interface{}{
		"tool_result":   summary,
		"changes":       changes,
		"changes_count": int64(len(result.Changes)),
		"success":       true,
		"error":         "",
	}, nil
}
