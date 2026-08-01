package account_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Create"
	Description  = "Create an account (company) in your Apollo CRM. Needs a name or domain."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+plus"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Account Name", Placeholder: "Analytical Engines Ltd"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "example.com (without www.)"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0000"},
	{Name: "raw_address", Type: core.ConnectionTypeString, Label: "Address", Placeholder: "London, United Kingdom"},
	{Name: "label_names", Type: core.ConnectionTypeString, Label: "List Names", Placeholder: "Target Accounts (comma-separated)"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"typed_custom_fields":{"…":"…"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Account"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	name := apollo_common.OptionalString("name", inputs)
	domain := apollo_common.OptionalString("domain", inputs)
	if name == "" && domain == "" {
		return apollo_common.ErrorResult("an account name or domain is required"), nil
	}

	body := map[string]interface{}{}
	apollo_common.SetString(body, "name", "name", inputs)
	apollo_common.SetString(body, "domain", "domain", inputs)
	apollo_common.SetString(body, "phone", "phone", inputs)
	apollo_common.SetString(body, "raw_address", "raw_address", inputs)
	apollo_common.SetList(body, "label_names", "label_names", inputs)

	extra, err := apollo_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	apollo_common.MergeFields(body, extra)

	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/accounts", body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	account := apollo_common.Obj(resp, "account")
	if account == nil {
		return apollo_common.ErrorResult("account was not created"), nil
	}
	return apollo_common.ObjectResult("", account, fmt.Sprintf("Created account %s", apollo_common.IDOf(account))), nil
}
