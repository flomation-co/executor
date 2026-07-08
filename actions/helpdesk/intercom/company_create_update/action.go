package helpdesk_intercom_company_create_update

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create or Update Company"
	Description  = "Create a company in Intercom, or update the existing one that matches your Company ID. A company only shows up in Intercom once at least one contact is attached to it."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+plus"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company ID (yours)", Placeholder: "Your own identifier for the company, e.g. its ID in your CRM — can't be changed later", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Acme Inc."},
	{Name: "plan", Type: core.ConnectionTypeString, Label: "Plan", Placeholder: "The plan they're on, e.g. Pro"},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size", Placeholder: "Number of employees, e.g. 50"},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://acme.com"},
	{Name: "industry", Type: core.ConnectionTypeString, Label: "Industry", Placeholder: "e.g. Software"},
	{Name: "monthly_spend", Type: core.ConnectionTypeInteger, Label: "Monthly Spend", Placeholder: "How much they pay you per month, whole numbers only, e.g. 490"},
	{Name: "remote_created_at", Type: core.ConnectionTypeDateTime, Label: "Created At (in your system)", Placeholder: "When the company was created on your side, e.g. 2026-07-08T09:00:00Z"},
	{Name: "custom_attributes", Type: core.ConnectionTypeObject, Label: "Custom Attributes (JSON)", Placeholder: `{"account_tier":"gold"} — attributes must already exist in Intercom (Settings → Data)`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other Intercom company field`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Company ID (Intercom)"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Company"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	companyID, err := intercom.RequiredString("company_id", inputs)
	if err != nil {
		return intercom.ErrorResult("provide your Company ID — Intercom uses it to decide whether to create the company or update an existing one"), nil
	}

	// POST /companies is an upsert keyed on YOUR company_id: found ⇒ update,
	// not found ⇒ create. The Intercom-side id comes back in the response.
	body := map[string]interface{}{"company_id": companyID}
	intercom.SetIfPresent(body, inputs, "name", "name")
	intercom.SetIfPresent(body, inputs, "plan", "plan")
	if err := intercom.SetIntIfPresent(body, inputs, "size", "size"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	intercom.SetIfPresent(body, inputs, "website", "website")
	intercom.SetIfPresent(body, inputs, "industry", "industry")
	// monthly_spend is integer-typed by Intercom (floats get truncated).
	if err := intercom.SetIntIfPresent(body, inputs, "monthly_spend", "monthly_spend"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.SetUnixIfPresent(body, inputs, "remote_created_at", "remote_created_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.SetJSONIfPresent(body, inputs, "custom_attributes", "custom_attributes"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.MergeAdditionalFields(body, inputs); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/companies", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label := intercom.OptionalString("name", inputs)
	if label == "" {
		label = companyID
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Saved company %s", label)), nil
}
