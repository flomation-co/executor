package marketing_sendgrid_template_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Templates"
	Description  = "Retrieve the transactional templates on your SendGrid account. Shows dynamic (handlebars) templates unless you also ask for legacy ones; tick Return All to fetch every page."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
	{
		Name:  "generations",
		Type:  core.ConnectionTypeString,
		Label: "Generations",
		Options: []core.ConnectionOption{
			{Name: "Dynamic", Value: "dynamic"},
			{Name: "Legacy", Value: "legacy"},
			{Name: "Legacy and Dynamic", Value: "legacy,dynamic"},
		},
		Placeholder: "Dynamic unless you also need your legacy templates",
	},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 100, up to 200 per page)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to fetch every page of templates"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Templates"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// SendGrid defaults generations to legacy, so dynamic is requested
	// explicitly; page_size is required by the endpoint and ListMarketing
	// always sends it.
	generations := sendgrid.OptionalString("generations", inputs)
	if generations == "" {
		generations = "dynamic"
	}
	query := url.Values{}
	query.Set("generations", generations)

	limit, _ := sendgrid.OptionalInt("limit", inputs)
	returnAll, _ := sendgrid.OptionalBoolSet("return_all", inputs)
	items, err := sendgrid.ListMarketing(auth, "/v3/templates", query, "result", limit, returnAll)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	return sendgrid.ListResult(items, len(items), fmt.Sprintf("Retrieved %d template(s)", len(items))), nil
}
