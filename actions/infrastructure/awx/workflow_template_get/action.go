// Package infrastructure_awx_workflow_template_get fetches one AWX / AAP workflow
// job template.
//
// The workflow serializer REMOVES execution_environment (an EE is meaningless for
// a workflow, which runs no playbook of its own), so this action does not emit
// one. What it does surface is survey_enabled and the seven ask_*_on_launch flags
// in the record, which are what tell you which fields Launch Workflow Template
// will actually accept.
package infrastructure_awx_workflow_template_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Workflow Template"
	Description  = "Fetch one workflow template and the fields it will let you override at launch."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+eye"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "awx_url", Type: core.ConnectionTypeString, Label: "AWX / AAP URL", Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "API Token (recommended)", Value: "token"},
		{Name: "Username & Password", Value: "basic"},
	}},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "awx_username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your AWX username", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "awx_password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "your AWX password — note some AWX installs disable password authentication", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"basic"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate"},
	{Name: "api_prefix", Type: core.ConnectionTypeString, Label: "API Path Prefix (advanced)", Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/)."},

	{Name: "workflow_template_id", Type: core.ConnectionTypeString, Label: "Workflow Template", Placeholder: "Pick the workflow template to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Workflow Template ID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "survey_enabled", Type: core.ConnectionTypeBoolean, Label: "Survey Enabled"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Workflow Template"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := awx.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := awx.Context()
	defer cancel()

	id, err := awx.RequiredInt("workflow_template_id", "Workflow Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	obj, err := awx.GetResource(ctx, auth, fmt.Sprintf("workflow_job_templates/%d/", id), nil)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	name := awx.StringField(obj, "name")
	surveyEnabled := awx.BoolField(obj, "survey_enabled")

	summary := fmt.Sprintf("Fetched workflow template %q (ID %d)", name, id)
	if surveyEnabled {
		summary += ". It has a survey — put the answers in Extra Variables / Survey Answers when you launch it."
	}

	out := awx.ObjectResult(obj, summary)
	out["name"] = name
	out["survey_enabled"] = surveyEnabled
	return out, nil
}
