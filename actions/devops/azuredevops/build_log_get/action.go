package devops_azuredevops_build_log_get

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Get Build Log"
	Description  = "Fetch a build's logs as text — the payload to paste into a chat message when a build fails. Leave Log ID blank to concatenate every log for the build in order; set it to fetch just one. Very large logs are truncated and flagged rather than silently clipped."
	Website      = "https://www.flomation.co"
	Icon         = "azure+file-lines"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

// maxLogs / maxLogBytes bound the whole-build log fetch. A build with hundreds
// of steps would otherwise cost hundreds of round trips, and a chatty test
// suite's logs can dwarf anything an operator can read or paste.
const (
	maxLogs     = 25
	maxLogBytes = 2 << 20 // 2 MB
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "build_id", Type: core.ConnectionTypeInteger, Label: "Build", Placeholder: "the build ID", Required: true},
	{Name: "log_id", Type: core.ConnectionTypeInteger, Label: "Log ID", Placeholder: "one log's ID — leave blank for the whole build's logs"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Build ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Logs"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Log Text"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	buildID, err := azuredevops.RequiredInt("build_id", "Build", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	base := fmt.Sprintf("%s/_apis/build/builds/%d/logs", azuredevops.ProjectPath(project), buildID)

	if logID, set := azuredevops.OptionalInt("log_id", inputs); set {
		resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
			Method: http.MethodGet,
			Path:   fmt.Sprintf("%s/%d", base, logID),
		})
		if err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		if err := azuredevops.CheckResponse(resp); err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		out := azuredevops.SuccessResult(strconv.Itoa(buildID),
			map[string]interface{}{"buildId": buildID, "logId": logID},
			fmt.Sprintf("Fetched log %d of build %d", logID, buildID))
		out["content"] = string(resp.Body)
		out["truncated"] = resp.Truncated
		return out, nil
	}

	// No log named: fetch the index, then each log in order. The caps are the
	// point — a failed build's logs can run to hundreds of megabytes, which is
	// neither pasteable nor something a one-shot executor should hold in memory.
	indexResp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodGet, Path: base})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(indexResp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	logs, err := azuredevops.DecodeList(indexResp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	var b strings.Builder
	truncated := false
	fetched := 0
	for _, entry := range logs {
		if fetched >= maxLogs {
			truncated = true
			break
		}
		obj, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id := azuredevops.IDOf(obj)
		if id == "" {
			continue
		}
		resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
			Method: http.MethodGet,
			Path:   base + "/" + id,
		})
		if err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		if err := azuredevops.CheckResponse(resp); err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		fetched++
		if resp.Truncated {
			truncated = true
		}
		b.WriteString("===== log " + id + " =====\n")
		b.Write(resp.Body)
		b.WriteString("\n")
		if b.Len() >= maxLogBytes {
			truncated = true
			break
		}
	}
	text := b.String()
	if len(text) > maxLogBytes {
		text = text[:maxLogBytes]
		truncated = true
	}

	summary := fmt.Sprintf("Fetched %d log(s) of build %d", fetched, buildID)
	if truncated {
		summary += " (truncated — fetch a single Log ID for the full text)"
	}
	out := azuredevops.SuccessResult(strconv.Itoa(buildID),
		map[string]interface{}{"buildId": buildID, "logs": logs},
		summary)
	out["content"] = text
	out["truncated"] = truncated
	return out, nil
}
