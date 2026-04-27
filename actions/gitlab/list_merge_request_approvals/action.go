package gitlab_list_merge_request_approvals

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List MR Approvals"
	Description  = "View current approvals on a GitLab merge request"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "approvals", Type: core.ConnectionTypeObject, Label: "Approvals (JSON)"},
	{Name: "approved", Type: core.ConnectionTypeBoolean, Label: "Approved"},
	{Name: "approvals_required", Type: core.ConnectionTypeInteger, Label: "Approvals Required"},
	{Name: "approvals_left", Type: core.ConnectionTypeInteger, Label: "Approvals Left"},
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

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/merge_requests/%s/approvals", iid), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get approvals: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var approvals struct {
		Approved          bool          `json:"approved"`
		ApprovalsRequired int           `json:"approvals_required"`
		ApprovalsLeft     int           `json:"approvals_left"`
		ApprovedBy        []interface{} `json:"approved_by"`
	}
	if err := json.Unmarshal(resp.Body, &approvals); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	status := "not approved"
	if approvals.Approved {
		status = "approved"
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("MR !%s: %s (%d/%d approvals)", iid, status, len(approvals.ApprovedBy), approvals.ApprovalsRequired),
		"approvals":          fullData,
		"approved":           approvals.Approved,
		"approvals_required": int64(approvals.ApprovalsRequired),
		"approvals_left":     int64(approvals.ApprovalsLeft),
		"success":            true,
		"error":              "",
	}, nil
}
