// Package appointment_update implements the Freshsales "Appointment: Update" action.
package appointment_update

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
	Name         = "Appointment: Update"
	Description  = "Update an existing Freshsales appointment."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+pencil"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Appointment ID", Placeholder: "12345", Required: true},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location", Placeholder: "The Cornerhouse, Chester"},
	{Name: "is_allday", Type: core.ConnectionTypeBoolean, Label: "All Day"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London"},
	{Name: "appointment_attendees_attributes", Type: core.ConnectionTypeText, Label: "Attendees (JSON)", Placeholder: `[{"attendee_type":"FdMultiuser","attendee_id":123}]`},
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
	freshsales_common.SetString(record, "location", "location", inputs)
	freshsales_common.SetBool(record, "is_allday", "is_allday", inputs)
	freshsales_common.SetString(record, "time_zone", "time_zone", inputs)
	if attendees, aerr := freshsales_common.ParseJSONArray("appointment_attendees_attributes", inputs); aerr == nil && attendees != nil {
		record["appointment_attendees_attributes"] = attendees
	}
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"appointment": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPut, "/appointments/"+idValue, payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "appointment")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Updated appointment %s", freshsales_common.NameOf(recordOut))), nil
}
