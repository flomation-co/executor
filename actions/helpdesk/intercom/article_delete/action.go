package helpdesk_intercom_article_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Delete Article"
	Description  = "Permanently delete a Help Center article from Intercom by its ID. This cannot be undone."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+trash"
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
	{Name: "article_id", Type: core.ConnectionTypeString, Label: "Article ID", Placeholder: "The article's ID, e.g. 6871119", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Article ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := intercom.RequiredString("article_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	if err := intercom.DeleteResource(auth, "/articles/"+url.PathEscape(id)); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.SuccessResult(id, nil, fmt.Sprintf("Deleted article %s", id)), nil
}
