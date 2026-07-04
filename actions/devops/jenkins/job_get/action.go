package devops_jenkins_job_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: Get Job"
	Description  = "Fetch a Jenkins job's details — its description, buildability, health, and last build references."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+eye"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
	{Name: "job", Type: core.ConnectionTypeString, Label: "Job", Placeholder: "job name (or folder/job)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
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

	resp, err := jenkins.Get(cfg, jenkins.JobPath(job)+"/api/json")
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if err := jenkins.CheckResponse(resp); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	obj, err := jenkins.DecodeObject(resp)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.ResourceResult(obj, fmt.Sprintf("Fetched job %q", job)), nil
}
