package devops_jenkins_build_stop

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
	Name         = "Jenkins: Stop Build"
	Description  = "Abort a running Jenkins build. Accepts a build number or a keyword like lastBuild."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+circle-stop"
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
	}},
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
	job, err := jenkins.RequiredString("job", "Job", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	build, err := jenkins.RequiredString("build_number", "Build", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}

	resp, err := jenkins.Post(cfg, jenkins.JobPath(job)+"/"+url.PathEscape(build)+"/stop", "", nil)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return jenkins.ErrorResult(fmt.Sprintf("No build %q found for job %q — nothing to stop", build, job)), nil
	}
	// stop answers 302 redirecting to the build page (followed to a 2xx).
	if err := jenkins.CheckResponse(resp, http.StatusFound); err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}
	return jenkins.SuccessResult(fmt.Sprintf("Stopped build %s of %q", build, job), nil), nil
}
