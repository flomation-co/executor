package devops_jenkins_build_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	jenkins "flomation.app/automate/executor/actions/devops/jenkins"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Jenkins: List Builds"
	Description  = "List a Jenkins job's builds with their number, result, timestamp, and duration."
	Website      = "https://www.flomation.co"
	Icon         = "jenkins+list"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

// defaultLimit caps the builds returned when "Return All" is off, matching
// n8n's default page size.
const defaultLimit = 50

var Inputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
	{Name: "job", Type: core.ConnectionTypeString, Label: "Job", Placeholder: "job name (or folder/job)", Required: true},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every build rather than a capped page"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max builds to return (default 50)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Builds"},
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
	job, err := jenkins.RequiredString("job", "Job", inputs)
	if err != nil {
		return jenkins.ErrorResult(err.Error()), nil
	}

	// Jenkins caps the `builds` tree field at the ~100 newest builds; only
	// `allBuilds` returns the full history (at the cost of loading it from disk,
	// which is why it's opt-in via Return All). Read whichever field we asked
	// for back out of the response.
	const fields = "[number,url,result,timestamp,duration,building,displayName,fullDisplayName]"
	returnAll := jenkins.OptionalBool("return_all", inputs)
	field := "builds"
	tree := field + fields
	if returnAll {
		field = "allBuilds"
		tree = field + fields
	} else {
		limit := defaultLimit
		if v, ok := jenkins.OptionalInt("limit", inputs); ok && v > 0 {
			limit = v
		}
		tree += "{0," + strconv.Itoa(limit) + "}"
	}

	q := url.Values{"tree": {tree}}
	resp, err := jenkins.Get(cfg, jenkins.JobPath(job)+"/api/json?"+q.Encode())
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
	builds, _ := obj[field].([]interface{})
	if builds == nil {
		builds = []interface{}{}
	}
	return jenkins.ListResult(builds, fmt.Sprintf("Found %d build(s) for %q", len(builds), job)), nil
}
