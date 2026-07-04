package devops_jenkins_job_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: List Jobs"
	Description  = "List the jobs on the Jenkins instance with their name, URL, and status colour."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+list"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Jobs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	cfg, err := jenkins.GetConfig(inputs)
	if err != nil {
		return nil, err
	}

	q := url.Values{"tree": {"jobs[name,url,color,buildable,_class]"}}
	resp, err := jenkins.Get(cfg, "/api/json?"+q.Encode())
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
	jobs, _ := obj["jobs"].([]interface{})
	if jobs == nil {
		jobs = []interface{}{}
	}
	return jenkins.ListResult(jobs, fmt.Sprintf("Found %d job(s)", len(jobs))), nil
}
