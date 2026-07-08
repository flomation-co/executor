package helpdesk_intercom_article_update

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Update Article"
	Description  = "Update a Help Center article in Intercom — change its title, content, author, state, or the collection it lives in. Only the fields you fill in are changed."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+pen"
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
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "A new title for the article"},
	{Name: "author_id", Type: core.ConnectionTypeString, Label: "Author", Placeholder: "Change the teammate shown as the article's author"},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Body", Placeholder: "Replacement article content — HTML formatting is supported"},
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
	{Name: "parent_id", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Move the article into this collection"},
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
	id, err := intercom.RequiredString("article_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	intercom.SetIfPresent(body, inputs, "title", "title")
	// author_id is typed as a JSON integer by Intercom — a string is rejected.
	intercom.SetNumericIDIfPresent(body, inputs, "author_id", "author_id")
	intercom.SetIfPresent(body, inputs, "body", "body")
	intercom.SetIfPresent(body, inputs, "description", "description")
	intercom.SetIfPresent(body, inputs, "state", "state")
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
	if len(body) == 0 {
		return intercom.ErrorResult("provide at least one field to update — Title, Author, Body, Description, State, or Collection"), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPut, "/articles/"+url.PathEscape(id), body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Updated article %s", id)), nil
}
