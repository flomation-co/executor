package devops_jenkins_job_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: Create Job"
	Description  = "Create a new Jenkins job from a config.xml definition. Tip: to get the XML of an existing job, add ‘config.xml’ to the end of its URL."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+plus"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
	{Name: "new_job", Type: core.ConnectionTypeString, Label: "New Job Name", Placeholder: "name for the new job", Required: true},
	{Name: "xml", Type: core.ConnectionTypeCode, Label: "Config XML", Placeholder: "<project>…</project>", Required: true},
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
	name, err := jenkins.RequiredString("new_job", "New Job Name", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	xml, err := jenkins.RequiredString("xml", "Config XML", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}

	q := url.Values{"name": {name}}
	resp, err := jenkins.Post(cfg, "/createItem?"+q.Encode(), "application/xml", []byte(xml))
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if err := jenkins.CheckResponse(resp); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.SuccessResult(fmt.Sprintf("Created job %q", name), nil), nil
}
