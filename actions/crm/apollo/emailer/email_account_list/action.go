package email_account_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email Accounts: List"
	Description  = "List the connected mailboxes in Apollo — use an id as the sequence sender. Master key."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+list"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Email Accounts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/email_accounts", nil)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	accounts := apollo_common.Arr(resp, "email_accounts")
	return apollo_common.ListResult(accounts, fmt.Sprintf("Found %d email account(s)", len(accounts))), nil
}
