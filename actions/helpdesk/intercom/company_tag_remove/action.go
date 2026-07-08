package helpdesk_intercom_company_tag_remove

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Remove Tag from Company"
	Description  = "Remove a tag from a company in Intercom. The tag itself is kept, so it stays available for other companies and contacts."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+minus"
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
	{Name: "tag_name", Type: core.ConnectionTypeString, Label: "Tag Name", Placeholder: "The exact name of the tag to remove, e.g. VIP customer", Required: true},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company", Placeholder: "The company to remove the tag from — Intercom's company ID, not your own", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Tag"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	tagName, err := intercom.RequiredString("tag_name", inputs)
	if err != nil {
		return intercom.ErrorResult("provide the name of the tag to remove"), nil
	}
	companyID, err := intercom.RequiredString("company_id", inputs)
	if err != nil {
		return intercom.ErrorResult("pick the company to remove the tag from"), nil
	}

	// Company untagging is the same POST /tags call as tagging, with
	// "untag": true on the company entry — not a DELETE.
	body := map[string]interface{}{
		"name":      tagName,
		"companies": []interface{}{map[string]interface{}{"id": companyID, "untag": true}},
	}
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/tags", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Removed tag %q from company %s", tagName, companyID)), nil
}
