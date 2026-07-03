package scheduling_acuity_appointment_type_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Many Appointment Types"
	Description  = "List your Acuity appointment types (the bookable services and classes)."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+layer-group"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted", Placeholder: "Also return deleted appointment types"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Appointment Types"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	userID, apiKey, err := acuity.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	if acuity.OptionalBool("include_deleted", inputs) {
		q.Set("includeDeleted", "true")
	}

	items, err := acuity.GetList(userID, apiKey, "/appointment-types", q)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Retrieved %d appointment types", len(items))), nil
}
