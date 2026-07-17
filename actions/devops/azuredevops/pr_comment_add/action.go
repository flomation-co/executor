package devops_azuredevops_pr_comment_add

import (
	"fmt"
	"net/http"
	"strconv"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Comment on Pull Request"
	Description  = "Comment on a pull request. Azure DevOps models PR comments as threads, so a new comment starts a thread; give a Thread ID to reply inside an existing one instead."
	Website      = "https://www.flomation.co"
	Icon         = "azure+comment"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "pull_request_id", Type: core.ConnectionTypeInteger, Label: "Pull Request", Placeholder: "the PR ID, e.g. 128", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Comment", Placeholder: "what you want to say — Markdown is supported", Required: true},
	{Name: "thread_id", Type: core.ConnectionTypeInteger, Label: "Reply to Thread", Placeholder: "an existing thread's ID — leave blank to start a new thread"},
	{Name: "thread_status", Type: core.ConnectionTypeString, Label: "Thread Status", Placeholder: "default active — only applies to a new thread", Options: []core.ConnectionOption{{Name: "Active", Value: "active"}, {Name: "Fixed", Value: "fixed"}, {Name: "Won't Fix", Value: "wontFix"}, {Name: "Closed", Value: "closed"}, {Name: "Pending", Value: "pending"}}, Visible: &core.VisibleWhen{Field: "thread_id", Values: []string{""}}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Thread ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Thread"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azuredevops.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	project, err := azuredevops.RequiredString("project", "Project", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	repo, err := azuredevops.RequiredString("repository", "Repository", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	prID, err := azuredevops.RequiredInt("pull_request_id", "Pull Request", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	content, err := azuredevops.RequiredString("content", "Comment", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	base := fmt.Sprintf("%s/_apis/git/repositories%s/pullRequests/%d/threads", azuredevops.ProjectPath(project), azuredevops.ProjectPath(repo), prID)

	// There is no flat "add a comment" endpoint: a top-level comment IS a new
	// thread, and a reply is a comment posted into an existing one. The two are
	// different URLs and different bodies.
	if threadID, set := azuredevops.OptionalInt("thread_id", inputs); set {
		resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
			Method: http.MethodPost,
			Path:   fmt.Sprintf("%s/%d/comments", base, threadID),
			Body:   map[string]interface{}{"content": content, "commentType": "text"},
		})
		if err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		if err := azuredevops.CheckResponse(resp); err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		obj, err := azuredevops.Decode(resp)
		if err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		out := azuredevops.ResourceResult(obj, fmt.Sprintf("Replied in thread %d on pull request %d", threadID, prID))
		out["id"] = strconv.Itoa(threadID)
		return out, nil
	}

	body := map[string]interface{}{
		"comments": []interface{}{map[string]interface{}{"content": content, "commentType": "text"}},
	}
	status := azuredevops.OptionalString("thread_status", inputs)
	if status == "" {
		status = "active"
	}
	body["status"] = status

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodPost, Path: base, Body: body})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	obj, err := azuredevops.Decode(resp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	return azuredevops.ResourceResult(obj, fmt.Sprintf("Commented on pull request %d", prID)), nil
}
