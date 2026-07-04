package devops_jenkins_build_console

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: Get Build Console Output"
	Description  = "Fetch the console log (build output) of a Jenkins build as plain text. Accepts a build number or a keyword like lastBuild."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+terminal"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
	{Name: "job", Type: core.ConnectionTypeString, Label: "Job", Placeholder: "job name (or folder/job)", Required: true},
	{Name: "build_number", Type: core.ConnectionTypeString, Label: "Build", Placeholder: "42, or lastBuild", Required: true, Options: []core.ConnectionOption{
		{Name: "Last Build", Value: "lastBuild"},
		{Name: "Last Successful Build", Value: "lastSuccessfulBuild"},
		{Name: "Last Failed Build", Value: "lastFailedBuild"},
		{Name: "Last Completed Build", Value: "lastCompletedBuild"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "console", Type: core.ConnectionTypeText, Label: "Console Output"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	cfg, err := jenkins.GetConfig(inputs)
	if err != nil {
		return nil, err
	}
	job, err := jenkins.RequiredString("job", "Job", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	build, err := jenkins.RequiredString("build_number", "Build", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}

	resp, err := jenkins.Get(cfg, jenkins.JobPath(job)+"/"+url.PathEscape(build)+"/consoleText")
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return jenkins.ErrorResult(fmt.Sprintf("No build %q found for job %q — the job may not have run yet", build, job)), nil
	}
	if err := jenkins.CheckResponse(resp); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	console := string(resp.Body)
	summary := fmt.Sprintf("Fetched console output for build %s of %q (%d bytes)", build, job, len(console))
	if resp.Truncated {
		// The tail of a Jenkins log carries the terminal "Finished: SUCCESS/
		// FAILURE" line, so flag the clip rather than returning a log that looks
		// complete but silently lost its ending.
		console += "\n…[truncated by Flomation at 8 MB — the console log was longer]"
		summary += " — truncated at 8 MB"
	}
	return jenkins.SuccessResult(summary, map[string]interface{}{
		"console":   console,
		"truncated": resp.Truncated,
	}), nil
}
