// Package account_clone implements the Freshsales "Account: Clone" action.
package account_clone

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
	Name         = "Account: Clone"
	Description  = "Duplicate an existing Freshsales account."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+copy"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "12345", Required: true},
	{Name: "website", Type: core.ConnectionTypeString, Label: "Website", Placeholder: "https://example.com"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0000"},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "industry_type_id", Type: core.ConnectionTypeInteger, Label: "Industry Type ID"},
	{Name: "business_type_id", Type: core.ConnectionTypeInteger, Label: "Business Type ID"},
	{Name: "territory_id", Type: core.ConnectionTypeInteger, Label: "Territory ID"},
	{Name: "parent_sales_account_id", Type: core.ConnectionTypeInteger, Label: "Parent Account ID"},
	{Name: "address", Type: core.ConnectionTypeString, Label: "Address"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "County / State"},
	{Name: "zipcode", Type: core.ConnectionTypeString, Label: "Postcode"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country"},
	{Name: "number_of_employees", Type: core.ConnectionTypeInteger, Label: "Employees"},
	{Name: "annual_revenue", Type: core.ConnectionTypeString, Label: "Annual Revenue", Placeholder: "1000000"},
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

	idValue, err := freshsales_common.RequiredString("id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	record := map[string]interface{}{}
	freshsales_common.SetString(record, "website", "website", inputs)
	freshsales_common.SetString(record, "phone", "phone", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetInt(record, "industry_type_id", "industry_type_id", inputs)
	freshsales_common.SetInt(record, "business_type_id", "business_type_id", inputs)
	freshsales_common.SetInt(record, "territory_id", "territory_id", inputs)
	freshsales_common.SetInt(record, "parent_sales_account_id", "parent_sales_account_id", inputs)
	freshsales_common.SetString(record, "address", "address", inputs)
	freshsales_common.SetString(record, "city", "city", inputs)
	freshsales_common.SetString(record, "state", "state", inputs)
	freshsales_common.SetString(record, "zipcode", "zipcode", inputs)
	freshsales_common.SetString(record, "country", "country", inputs)
	freshsales_common.SetInt(record, "number_of_employees", "number_of_employees", inputs)
	freshsales_common.SetNumber(record, "annual_revenue", "annual_revenue", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"account": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/sales_accounts/"+idValue+"/clone", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "account")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Cloned account %s", freshsales_common.NameOf(recordOut))), nil
}
