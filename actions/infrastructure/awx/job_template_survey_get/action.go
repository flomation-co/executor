// Package infrastructure_awx_job_template_survey_get fetches a job template's
// survey — the questions AWX asks the person launching it.
//
// ★ THE TRAP: a template with NO SURVEY answers this endpoint with HTTP 200 AND
// AN EMPTY OBJECT — not a 404, not an error. A naive client reports a successful
// fetch of an empty survey and the operator is left staring at a blank result
// wondering what broke. This action says so in words: has_survey=false, a
// tool_result that explains there are no questions, and success=true, because
// nothing failed.
//
// Two shapes are normalised for the flow:
//
//   - `choices` comes back from AWX as a NEWLINE-SEPARATED STRING
//     ("dev\nstaging\nprod") rather than an array. awx.FetchSurveySpec splits it,
//     and this action emits it as a real array so a downstream Loop node can
//     iterate the options.
//   - a `password` question's default reads back as the literal "$encrypted$" —
//     AWX never returns the stored value.
package infrastructure_awx_job_template_survey_get

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "AWX: Get Job Template Survey"
	Description  = "Fetch a job template's survey — the questions it asks at launch, with their types, defaults and allowed choices. Use it to build a form for the operator."
	Website      = "https://www.flomation.co"
	Icon         = "ansible+file-lines"
	Date         = "14/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// ---- AUTH (the shared block — see awx.AuthInputs) ----
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

	{Name: "job_template_id", Type: core.ConnectionTypeString, Label: "Job Template", Placeholder: "Pick a job template, or enter its AWX ID (e.g. 7)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Survey"},
	{Name: "spec", Type: core.ConnectionTypeObject, Label: "Questions"},
	{Name: "has_survey", Type: core.ConnectionTypeBoolean, Label: "Has Survey"},
	{Name: "required_variables", Type: core.ConnectionTypeObject, Label: "Required Variables"},
	{Name: "question_count", Type: core.ConnectionTypeInteger, Label: "Question Count"},
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

	id, err := awx.RequiredInt("job_template_id", "Job Template", inputs)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	spec, err := awx.FetchSurveySpec(ctx, auth, awx.TemplateKindJob, id)
	if err != nil {
		return awx.ErrorResult(err.Error()), nil
	}

	questions := make([]interface{}, 0, len(spec.Spec))
	for _, q := range spec.Spec {
		questions = append(questions, question(q))
	}
	required := strings2iface(spec.RequiredVariables())

	raw := spec.Raw
	if raw == nil {
		raw = map[string]interface{}{}
	}

	return map[string]interface{}{
		"result":             raw,
		"spec":               questions,
		"has_survey":         spec.HasSurvey(),
		"required_variables": required,
		"question_count":     len(questions),
		"tool_result":        summarise(id, spec),
		// Not an error. An empty survey is AWX's honest answer to "what do you
		// ask?" — "nothing" — and the flow should carry on.
		"success": true,
		"error":   "",
	}, nil
}

// question renders one survey question as a plain JSON object, so a downstream
// Loop node can iterate the spec and a form can be built from it. min/max are
// omitted rather than emitted as null, because AWX only sets them on the types
// that have them (a length bound on text, a value bound on integer/float).
func question(q awx.SurveyQuestion) map[string]interface{} {
	out := map[string]interface{}{
		"variable":      q.Variable,
		"question_name": q.QuestionName,
		"type":          q.Type,
		"required":      q.Required,
		"default":       q.Default,
		"choices":       strings2iface(q.Choices),
	}
	if q.QuestionDescription != "" {
		out["question_description"] = q.QuestionDescription
	}
	if q.Min != nil {
		out["min"] = *q.Min
	}
	if q.Max != nil {
		out["max"] = *q.Max
	}
	return out
}

func summarise(id int64, spec awx.SurveySpec) string {
	if !spec.HasSurvey() {
		return fmt.Sprintf(
			"Job template %d has NO SURVEY configured. (AWX answers this with an empty result and HTTP 200 — nothing has failed.) "+
				"There are no questions to answer: launch it with Extra Variables / Survey Answers left blank, unless the template prompts for variables in its own right.", id)
	}

	var b strings.Builder
	if name := strings.TrimSpace(spec.Name); name != "" {
		fmt.Fprintf(&b, "Survey %q on job template %d: %d question(s). ", name, id, len(spec.Spec))
	} else {
		fmt.Fprintf(&b, "Job template %d has a survey with %d question(s). ", id, len(spec.Spec))
	}

	lines := make([]string, 0, len(spec.Spec))
	for _, q := range spec.Spec {
		line := fmt.Sprintf("%s (%s", q.Variable, q.Type)
		if q.Required {
			line += ", required"
		}
		if len(q.Choices) > 0 {
			line += ", one of: " + strings.Join(q.Choices, " / ")
		}
		lines = append(lines, line+")")
	}
	b.WriteString(strings.Join(lines, "; "))
	b.WriteString(". Answer them on the Launch Job Template node in Extra Variables / Survey Answers, keyed by the variable name.")

	if required := spec.RequiredVariables(); len(required) > 0 {
		fmt.Fprintf(&b, " These have no default and MUST be answered: %s.", strings.Join(required, ", "))
	}
	if hasPassword(spec) {
		b.WriteString(" Note a password question's default reads back as the literal \"$encrypted$\" — AWX never returns the stored value.")
	}
	return b.String()
}

func hasPassword(spec awx.SurveySpec) bool {
	for _, q := range spec.Spec {
		if q.Type == "password" {
			return true
		}
	}
	return false
}

// strings2iface widens a []string to the []interface{} the flow engine's Loop
// node iterates. An empty list stays an empty ARRAY, never null.
func strings2iface(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}
