package organization_enrich

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Company: Enrich"
	Description  = "Enrich a company in Apollo by its domain. Returns the organisation object."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+bolt"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Company Domain", Placeholder: "example.com (without www.)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Organisation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Organisation"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	domain, err := apollo_common.RequiredString("domain", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a company domain is required"), nil
	}

	q := url.Values{}
	q.Set("domain", domain)

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/organizations/enrich", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	org := apollo_common.Obj(resp, "organization")
	if org == nil {
		return apollo_common.ErrorResult(fmt.Sprintf("no organisation found for %s", domain)), nil
	}
	name, _ := org["name"].(string)
	return apollo_common.ObjectResult("", org, fmt.Sprintf("Enriched %s", name)), nil
}
