package organization_bulk_enrich

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Company: Bulk Enrich"
	Description  = "Enrich up to 10 companies at once from a comma-separated list of domains."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+bolt"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "domains", Type: core.ConnectionTypeString, Label: "Domains", Placeholder: "example.com, another.com (comma-separated, without www.)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Organisations"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	domains := apollo_common.StringList("domains", inputs)
	if len(domains) == 0 {
		return apollo_common.ErrorResult("at least one domain is required"), nil
	}

	q := url.Values{}
	for _, d := range domains {
		q.Add("domains[]", d)
	}

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/organizations/bulk_enrich", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	orgs := apollo_common.Arr(resp, "organizations")
	return apollo_common.ListResult(orgs, fmt.Sprintf("Enriched %d companies", len(orgs))), nil
}
