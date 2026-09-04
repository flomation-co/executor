// Package contact_create implements the Freshsales "Contact: Create" action.
package contact_create

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: Create"
	Description  = "Create a contact in Freshsales. Returns the new record and its ID."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+plus"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Ada"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Lovelace"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "ada@example.com"},
	{Name: "mobile_number", Type: core.ConnectionTypeString, Label: "Mobile", Placeholder: "+44 7700 900000"},
	{Name: "work_number", Type: core.ConnectionTypeString, Label: "Work Phone", Placeholder: "+44 20 7946 0000"},
	{Name: "job_title", Type: core.ConnectionTypeString, Label: "Job Title", Placeholder: "Head of Engineering"},
	{Name: "company_name", Type: core.ConnectionTypeString, Label: "Company Name", Placeholder: "Analytical Engines Ltd"},
	{Name: "sales_account_id", Type: core.ConnectionTypeInteger, Label: "Account ID", Placeholder: "The sales account to link this contact to"},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID", Placeholder: "Sales rep user id — see Settings: Get Selector"},
	{Name: "lead_source_id", Type: core.ConnectionTypeInteger, Label: "Lead Source ID"},
	{Name: "lifecycle_stage_id", Type: core.ConnectionTypeInteger, Label: "Lifecycle Stage ID"},
	{Name: "contact_status_id", Type: core.ConnectionTypeInteger, Label: "Contact Status ID"},
	{Name: "territory_id", Type: core.ConnectionTypeInteger, Label: "Territory ID"},
	{Name: "address", Type: core.ConnectionTypeString, Label: "Address", Placeholder: "1 Hoxton Square"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City", Placeholder: "London"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "County / State", Placeholder: "Greater London"},
	{Name: "zipcode", Type: core.ConnectionTypeString, Label: "Postcode", Placeholder: "N1 6NU"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "Your own system id"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"custom_field":{"cf_region":"EMEA"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	client, err := freshsales_common.Client(inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	record := map[string]interface{}{}
	freshsales_common.SetString(record, "first_name", "first_name", inputs)
	freshsales_common.SetString(record, "last_name", "last_name", inputs)
	freshsales_common.SetString(record, "email", "email", inputs)
	freshsales_common.SetString(record, "mobile_number", "mobile_number", inputs)
	freshsales_common.SetString(record, "work_number", "work_number", inputs)
	freshsales_common.SetString(record, "job_title", "job_title", inputs)
	freshsales_common.SetString(record, "company_name", "company_name", inputs)
	freshsales_common.SetInt(record, "sales_account_id", "sales_account_id", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetInt(record, "lead_source_id", "lead_source_id", inputs)
	freshsales_common.SetInt(record, "lifecycle_stage_id", "lifecycle_stage_id", inputs)
	freshsales_common.SetInt(record, "contact_status_id", "contact_status_id", inputs)
	freshsales_common.SetInt(record, "territory_id", "territory_id", inputs)
	freshsales_common.SetString(record, "address", "address", inputs)
	freshsales_common.SetString(record, "city", "city", inputs)
	freshsales_common.SetString(record, "state", "state", inputs)
	freshsales_common.SetString(record, "zipcode", "zipcode", inputs)
	freshsales_common.SetString(record, "country", "country", inputs)
	freshsales_common.SetString(record, "external_id", "external_id", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"contact": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/contacts", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "contact")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Created contact %s", freshsales_common.NameOf(recordOut))), nil
}
