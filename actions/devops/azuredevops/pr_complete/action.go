package devops_azuredevops_pr_complete

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Complete Pull Request"
	Description  = "Complete (merge) a pull request, with the usual options — squash, delete the source branch, resolve linked work items. Azure DevOps refuses to merge a PR that has moved since you last read it, so this action re-reads the PR first and echoes its latest commit back; no extra step needed."
	Website      = "https://www.flomation.co"
	Icon         = "azure+check"
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
	{Name: "merge_strategy", Type: core.ConnectionTypeString, Label: "Merge Strategy", Placeholder: "default: a merge commit", Options: []core.ConnectionOption{{Name: "Merge Commit", Value: "noFastForward"}, {Name: "Squash", Value: "squash"}, {Name: "Rebase", Value: "rebase"}, {Name: "Semi-Linear Merge", Value: "rebaseMerge"}}},
	{Name: "commit_message", Type: core.ConnectionTypeText, Label: "Merge Commit Message", Placeholder: "leave blank for the default message"},
	{Name: "delete_source_branch", Type: core.ConnectionTypeBoolean, Label: "Delete Source Branch", Placeholder: "remove the source branch once merged"},
	{Name: "transition_work_items", Type: core.ConnectionTypeBoolean, Label: "Resolve Linked Work Items", Placeholder: "move the PR's linked work items to their next state"},
	{Name: "bypass_policy", Type: core.ConnectionTypeBoolean, Label: "Bypass Branch Policies", Placeholder: "merge despite failing policies (needs elevated permission)"},
	{Name: "bypass_reason", Type: core.ConnectionTypeString, Label: "Bypass Reason", Placeholder: "why the policies were bypassed — recorded in the audit log", Visible: &core.VisibleWhen{Field: "bypass_policy", Values: []string{"true"}}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pull Request ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Pull Request"},
	{Name: "pr_status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "merge_status", Type: core.ConnectionTypeString, Label: "Merge Status"},
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
	path := fmt.Sprintf("%s/_apis/git/repositories%s/pullrequests/%d", azuredevops.ProjectPath(project), azuredevops.ProjectPath(repo), prID)

	// Completing a PR is inherently two calls. Azure DevOps requires
	// lastMergeSourceCommit to be echoed back on the PATCH — an optimistic
	// concurrency guard against merging a PR that has been pushed to since you
	// looked at it. Omit it and the merge is refused, so this reads it first
	// rather than making the operator wire a Get Pull Request node in front.
	getResp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodGet, Path: path})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(getResp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	current, err := azuredevops.Decode(getResp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	lastCommit, ok := current["lastMergeSourceCommit"].(map[string]interface{})
	if !ok {
		return azuredevops.ErrorResult(fmt.Sprintf("pull request %d has no last source commit — it may already be completed or abandoned", prID)), nil
	}
	if status, _ := current["status"].(string); status != "" && status != "active" {
		return azuredevops.ErrorResult(fmt.Sprintf("pull request %d is %s, not active — only an active pull request can be completed", prID, status)), nil
	}

	completion := map[string]interface{}{}
	if v := azuredevops.OptionalString("merge_strategy", inputs); v != "" {
		completion["mergeStrategy"] = v
	}
	azuredevops.SetIfPresent(completion, inputs, "mergeCommitMessage", "commit_message")
	azuredevops.SetBoolIfSet(completion, inputs, "deleteSourceBranch", "delete_source_branch")
	azuredevops.SetBoolIfSet(completion, inputs, "transitionWorkItems", "transition_work_items")
	azuredevops.SetBoolIfSet(completion, inputs, "bypassPolicy", "bypass_policy")
	azuredevops.SetIfPresent(completion, inputs, "bypassReason", "bypass_reason")

	body := map[string]interface{}{
		"status":                "completed",
		"lastMergeSourceCommit": lastCommit,
	}
	if len(completion) > 0 {
		body["completionOptions"] = completion
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodPatch, Path: path, Body: body})
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

	// The PATCH above only REQUESTS the merge — see awaitMerge.
	obj, err = awaitMerge(flow, auth, path, prID, obj)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	status, _ := obj["status"].(string)
	mergeStatus, _ := obj["mergeStatus"].(string)
	target, _ := obj["targetRefName"].(string)
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Completed pull request %d — merged into %s",
		prID, strings.TrimPrefix(target, "refs/heads/")))
	out["id"] = strconv.Itoa(prID)
	out["pr_status"] = status
	out["merge_status"] = mergeStatus
	return out, nil
}

// mergeWait / mergePoll bound the wait for a queued merge. Azure DevOps lands
// one in a second or two; 30s is generous enough for a slow one without stalling
// a flow behind a service that is wedged.
const (
	mergeWait = 30 * time.Second
	mergePoll = time.Second
)

// awaitMerge waits for Azure DevOps to actually perform the merge, and reports
// what really happened rather than what was asked for.
//
// Completing a pull request is ASYNCHRONOUS, which the PATCH response hides
// well: it answers 200 with the pull request still "active" and its mergeStatus
// "queued", and the merge lands a second or two later. Returning that response
// as-is claimed "Completed pull request 128" while the pull request was, by the
// service's own account, still open — and the queued merge can still FAIL, on a
// conflict, a branch policy, or a push that raced in behind the merge check.
//
// That lie is the dangerous kind, because of what a flow does next: delete the
// source branch, transition the work items, tell the channel it shipped. All of
// it would have been acting on a merge that had not happened and might never.
func awaitMerge(flow *core.Flow, a azuredevops.Auth, path string, prID int, latest map[string]interface{}) (map[string]interface{}, error) {
	deadline := time.Now().Add(mergeWait)
	for {
		status, _ := latest["status"].(string)
		merge, _ := latest["mergeStatus"].(string)

		switch status {
		case "completed":
			return latest, nil
		case "abandoned":
			return nil, fmt.Errorf("pull request %d was abandoned while it was being completed", prID)
		}
		// A failing merge leaves the pull request ACTIVE and reports itself here,
		// so the merge status is the only place the failure is ever named.
		switch merge {
		case "conflicts":
			return nil, fmt.Errorf("pull request %d could not be merged: the source and target branches conflict and must be reconciled first", prID)
		case "rejectedByPolicy":
			return nil, fmt.Errorf("pull request %d could not be merged: a branch policy rejected it — turn on Bypass Branch Policies if you have permission to override", prID)
		case "failure":
			return nil, fmt.Errorf("pull request %d could not be merged: Azure DevOps reported the merge as failed", prID)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("pull request %d is still merging after %s (status %s, merge %s) — the merge was accepted but has not landed yet; check it with Get Pull Request",
				prID, mergeWait, status, merge)
		}
		if err := azuredevops.SleepOrCancel(flow, mergePoll); err != nil {
			return nil, fmt.Errorf("gave up waiting for pull request %d to merge: %w", prID, err)
		}

		resp, err := azuredevops.Do(flow, a, azuredevops.Request{Method: http.MethodGet, Path: path})
		if err != nil {
			return nil, err
		}
		if err := azuredevops.CheckResponse(resp); err != nil {
			return nil, err
		}
		if latest, err = azuredevops.Decode(resp); err != nil {
			return nil, err
		}
	}
}
