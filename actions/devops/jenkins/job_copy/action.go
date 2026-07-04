package devops_jenkins_job_copy

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
	Name         = "Jenkins: Copy Job"
	Description  = "Create a new Jenkins job by copying the configuration of an existing one."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+copy"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
	{Name: "job", Type: core.ConnectionTypeString, Label: "Source Job", Placeholder: "job to copy from", Required: true},
	{Name: "new_job", Type: core.ConnectionTypeString, Label: "New Job Name", Placeholder: "name for the copy", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	cfg, err := jenkins.GetConfig(inputs)
	if err != nil {
		return nil, err
	}
	source, err := jenkins.RequiredString("job", "Source Job", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	name, err := jenkins.RequiredString("new_job", "New Job Name", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}

	q := url.Values{"name": {name}, "mode": {"copy"}, "from": {source}}
	resp, err := jenkins.Post(cfg, "/createItem?"+q.Encode(), "", nil)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	// A successful copy answers 200, or 302 redirecting to the new job's page
	// (followed automatically by the HTTP client, landing back on a 2xx).
	if err := jenkins.CheckResponse(resp, http.StatusFound); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.SuccessResult(fmt.Sprintf("Copied job %q to %q", source, name), nil), nil
}
