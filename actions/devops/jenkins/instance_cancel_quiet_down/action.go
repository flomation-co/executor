package devops_jenkins_instance_cancel_quiet_down

import (
	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: Cancel Quiet Down"
	Description  = "Cancel quiet-down mode so Jenkins resumes starting new builds."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+check"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
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

	resp, err := jenkins.Post(cfg, "/cancelQuietDown", "", nil)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if err := jenkins.CheckResponse(resp); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.SuccessResult("Cancelled quiet-down — Jenkins will start new builds again", nil), nil
}
