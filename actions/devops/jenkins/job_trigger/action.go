package devops_jenkins_job_trigger

import (
	"fmt"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: Trigger Job"
	Description  = "Start a build of a Jenkins job. Returns the queue item URL so a later step can track it."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+play"
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
	{Name: "queue_url", Type: core.ConnectionTypeString, Label: "Queue Item URL"},
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

	resp, err := jenkins.Post(cfg, jenkins.JobPath(job)+"/build", "", nil)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if err := jenkins.CheckResponse(resp, 201); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.SuccessResult(fmt.Sprintf("Triggered job %q", job), map[string]interface{}{
		"queue_url": resp.Location,
	}), nil
}
