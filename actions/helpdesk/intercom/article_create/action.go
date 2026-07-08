package helpdesk_intercom_article_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create Article"
	Description  = "Create a Help Center article in Intercom. Articles start as drafts unless you set State to Published; the Body accepts HTML."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+plus"
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
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "How do I reset my password?", Required: true},
	{Name: "author_id", Type: core.ConnectionTypeString, Label: "Author", Placeholder: "The teammate shown as the article's author", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Placeholder: "The article content — HTML formatting is supported"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "A short summary shown in Help Center search results"},
	{
		Name:  "state",
		Type:  core.ConnectionTypeString,
		Label: "State",
		Options: []core.ConnectionOption{
			{Name: "Draft", Value: "draft"},
			{Name: "Published", Value: "published"},
		},
	},
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "The collection to file this article under"},
	{
		Name:  "parent_type",
		Type:  core.ConnectionTypeString,
		Label: "Parent Type",
		Options: []core.ConnectionOption{
			{Name: "Collection", Value: "collection"},
			{Name: "Section", Value: "section"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Article ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Article"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	title, err := intercom.RequiredString("title", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if _, err := intercom.RequiredString("author_id", inputs); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"title": title}
	// author_id is typed as a JSON integer by Intercom — a string is rejected.
	intercom.SetNumericIDIfPresent(body, inputs, "author_id", "author_id")
	intercom.SetIfPresent(body, inputs, "body", "body")
	intercom.SetIfPresent(body, inputs, "description", "description")
	state := intercom.OptionalString("state", inputs)
	if state == "" {
		state = "draft"
	}
	body["state"] = state
	intercom.SetNumericIDIfPresent(body, inputs, "parent_id", "parent_id")
	if _, ok := body["parent_id"]; ok {
		// The Collection dropdown lists collections, so an unset Parent Type
		// means collection; without it Intercom would reject the parent id.
		parentType := intercom.OptionalString("parent_type", inputs)
		if parentType == "" {
			parentType = "collection"
		}
		body["parent_type"] = parentType
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/articles", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Created %s article %q", state, title)), nil
}
